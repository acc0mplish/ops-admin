import http from './http'

// §4.8 one-time console tickets: minted per dial, consumed by the terminal
// websocket gate. Single use — every (re)connect and container switch mints
// a fresh ticket (30s TTL).
export const mintAssetHostTerminalTicket = (hostId) =>
  http.post('/api/v1/console-sessions', {
    resourceType: 'asset_host',
    resourceId: String(hostId),
    protocol: 'asset-terminal'
  })

export const mintK8sPodTerminalTicket = ({ clusterId, namespace, podName }) =>
  http.post('/api/v1/console-sessions', {
    resourceType: 'k8s_pod',
    resourceId: `${clusterId}/${namespace}/${podName}`,
    protocol: 'k8s-pod-terminal'
  })
