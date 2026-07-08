package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"

	"ops-admin/backend/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gorilla/websocket"
)

type k8sTerminalMessage struct {
	Operation string `json:"operation"`
	Data      any    `json:"data,omitempty"`
}

type k8sTerminalSizeData struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

type k8sTerminalSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func newK8sTerminalSizeQueue(rows int, cols int) *k8sTerminalSizeQueue {
	queue := &k8sTerminalSizeQueue{
		ch: make(chan remotecommand.TerminalSize, 8),
	}
	queue.Push(rows, cols)
	return queue
}

func (q *k8sTerminalSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return &size
}

func (q *k8sTerminalSizeQueue) Push(rows int, cols int) {
	if q == nil || rows <= 0 || cols <= 0 {
		return
	}
	size := remotecommand.TerminalSize{
		Width:  uint16(cols),
		Height: uint16(rows),
	}
	select {
	case q.ch <- size:
	default:
		select {
		case <-q.ch:
		default:
		}
		q.ch <- size
	}
}

func (q *k8sTerminalSizeQueue) Close() {
	if q == nil {
		return
	}
	close(q.ch)
}

type k8sTerminalOutput struct {
	conn    *websocket.Conn
	writeMu *sync.Mutex
}

func (w *k8sTerminalOutput) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if err := w.conn.WriteJSON(k8sTerminalMessage{
		Operation: "stdout",
		Data:      string(p),
	}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *Service) GetK8sPodContainers(clusterID uint, namespace string, podName string) ([]string, error) {
	clientset, err := s.newK8sClientset(clusterID)
	if err != nil {
		return nil, err
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	containers := make([]string, 0, len(pod.Spec.Containers))
	for _, item := range pod.Spec.Containers {
		containers = append(containers, item.Name)
	}
	return containers, nil
}

func (s *Service) OpenK8sPodTerminal(clusterID uint, namespace string, podName string, container string, command string, rows int, cols int, conn *websocket.Conn) error {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return err
	}
	config, cleanup, err := s.k8sRESTConfigForCluster(cluster)
	if err != nil {
		return errors.New(k8sClusterConnectError)
	}
	defer cleanup()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.New(k8sClusterConnectError)
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	targetContainer := chooseK8sContainerName(pod, container)
	if targetContainer == "" {
		return errors.New("pod container not found")
	}

	cmd := normalizeK8sTerminalCommand(command)
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: targetContainer,
		Command:   cmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()
	defer stdinWriter.Close()

	sizeQueue := newK8sTerminalSizeQueue(rows, cols)
	defer sizeQueue.Close()

	writeMu := &sync.Mutex{}
	streamErrCh := make(chan error, 1)
	readErrCh := make(chan error, 1)

	go func() {
		streamErrCh <- executor.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             stdinReader,
			Stdout:            &k8sTerminalOutput{conn: conn, writeMu: writeMu},
			Stderr:            &k8sTerminalOutput{conn: conn, writeMu: writeMu},
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		})
	}()

	go func() {
		defer stdinWriter.Close()
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				readErrCh <- err
				return
			}

			var message k8sTerminalMessage
			if err := json.Unmarshal(payload, &message); err != nil {
				if _, writeErr := stdinWriter.Write(payload); writeErr != nil {
					readErrCh <- writeErr
					return
				}
				continue
			}

			switch message.Operation {
			case "stdin":
				text, _ := message.Data.(string)
				if text == "" {
					continue
				}
				if _, err := stdinWriter.Write([]byte(text)); err != nil {
					readErrCh <- err
					return
				}
			case "resize":
				var size k8sTerminalSizeData
				if raw, err := json.Marshal(message.Data); err == nil && json.Unmarshal(raw, &size) == nil {
					sizeQueue.Push(int(size.Rows), int(size.Cols))
				}
			}
		}
	}()

	select {
	case err := <-streamErrCh:
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	case err := <-readErrCh:
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil
		}
		return nil
	}
}

func (s *Service) newK8sClientset(clusterID uint) (*kubernetes.Clientset, error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		return nil, err
	}
	config, cleanup, err := s.k8sRESTConfigForCluster(cluster)
	if err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	defer cleanup()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	return clientset, nil
}

func (s *Service) k8sRESTConfigForCluster(cluster model.K8sCluster) (*rest.Config, func(), error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.KubeConfig))
	if err != nil {
		return nil, func() {}, err
	}
	if normalizeConnectionMode(cluster.ConnectionMode) == "gateway" && cluster.GatewayID != nil && *cluster.GatewayID > 0 {
		gatewayID := *cluster.GatewayID
		config.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, cleanup, err := s.dialThroughGateway(ctx, gatewayID, network, address)
			if err != nil {
				return nil, err
			}
			return cleanupConn{Conn: conn, cleanup: cleanup}, nil
		}
	}
	return config, func() {}, nil
}

func chooseK8sContainerName(pod *corev1.Pod, container string) string {
	name := Trimmed(container)
	if name != "" {
		for _, item := range pod.Spec.Containers {
			if item.Name == name {
				return item.Name
			}
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	return ""
}

func normalizeK8sTerminalCommand(command string) []string {
	switch Trimmed(command) {
	case "", "sh", "/bin/sh":
		return []string{"/bin/sh"}
	case "bash", "/bin/bash":
		return []string{"/bin/bash"}
	default:
		return []string{Trimmed(command)}
	}
}
