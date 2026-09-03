#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import sys

REPLACEMENTS: dict[str, list[tuple[str, str]]] = {
    "backend/service/database.go": [
        ('"当前数据库为只读模式，禁止新增、编辑、删除或导入数据"', '"the current database is read-only; creating, editing, deleting, or importing data is not allowed"'),
        ('"数据库名称不能为空"', '"database name is required"'),
        ('"数据库地址不能为空"', '"database address is required"'),
        ('"数据库用户名不能为空"', '"database username is required"'),
        ('"请选择所属环境"', '"select an environment"'),
        ('"新增数据库资产"', '"Create Database Asset"'),
        ('"更新数据库基础信息"', '"Update Database Details"'),
        ('"删除数据库资产"', '"Delete Database Asset"'),
        ('"请输入数据库名称"', '"enter a database name"'),
        ('"名称须以字母或下划线开头，仅可包含字母、数字和下划线，长度不超过 63 个字符"', '"name must start with a letter or underscore, contain only letters, digits, and underscores, and be at most 63 characters"'),
        ('"字符集或排序规则格式不正确"', '"invalid character set or collation format"'),
        ('"排序规则必须与所选字符集匹配"', '"collation must match the selected character set"'),
        ('"当前数据库类型不支持新增数据库"', '"the current database type does not support creating databases"'),
        ('"SQL 工作台新增数据库："', '"SQL Workbench Create Database: "'),
        ('"SQL 工作台新增 PostgreSQL Schema："', '"SQL Workbench Create PostgreSQL Schema: "'),
        ('"当前数据库类型不支持字符集与排序规则设置"', '"the current database type does not support character-set or collation settings"'),
        ('"请先选择数据表"', '"select a table first"'),
        ('"请先选择表"', '"select a table first"'),
        ('"请输入 SQL"', '"enter SQL"'),
        ('"当前数据库为只读模式，禁止执行写入或结构变更 SQL"', '"the current database is read-only; write or schema-changing SQL is not allowed"'),
        ('"写操作必须完成执行前确认"', '"write operations require confirmation before execution"'),
        ('"请输入有效 SQL"', '"enter valid SQL"'),
        ('statementType+" 属于高风险结构或权限变更"', 'statementType+" is a high-risk schema or privilege change"'),
        ('statementType+" 未包含 WHERE 条件"', 'statementType+" does not contain a WHERE clause"'),
        ('"包含无法确认为只读的 SQL 语句"', '"contains SQL that cannot be verified as read-only"'),
        ('fmt.Sprintf("将连续执行 %d 条 SQL", len(statements))', 'fmt.Sprintf("will execute %d SQL statements sequentially", len(statements))'),
        ('"目标数据库属于生产环境"', '"target database belongs to the production environment"'),
        ('"未发现明显高风险特征"', '"no obvious high-risk characteristics detected"'),
        ('"没有可插入的数据"', '"no data is available to insert"'),
        ('"当前表没有主键，禁止直接编辑结果集"', '"the current table has no primary key; direct result-set editing is not allowed"'),
        ('"当前表没有主键，禁止删除结果集数据"', '"the current table has no primary key; deleting result-set rows is not allowed"'),
        ('"获取建表语句失败"', '"failed to retrieve the CREATE TABLE statement"'),
        ('"请选择源数据库和目标数据库"', '"select source and target databases"'),
        ('"请选择源表"', '"select a source table"'),
        ('"跨数据库类型导入暂不支持自动建表，请先创建目标表后再导入"', '"automatic table creation is not supported for cross-database-type imports; create the target table first"'),
        ('"获取源表结构失败"', '"failed to retrieve source-table schema"'),
        ('"源表和目标表没有可匹配的字段"', '"source and target tables have no matching columns"'),
        ('"请先选择要导出的表"', '"select tables to export first"'),
        ('"等待执行"', '"Pending execution"'),
        ('"导入预检查未通过，请处理风险项后重试"', '"import precheck failed; resolve the risk items and retry"'),
        ('"请选择源数据库、源表和目标数据库"', '"select a source database, source table, and target database"'),
        ('"源表不存在或没有可导入字段"', '"source table does not exist or has no importable columns"'),
        ('"目标数据库为只读模式"', '"target database is read-only"'),
        ('"目标表不存在，且未启用自动建表"', '"target table does not exist and automatic table creation is disabled"'),
        ('"跨数据库类型导入不支持自动建表，请先创建目标表"', '"cross-database-type imports do not support automatic table creation; create the target table first"'),
        ('"导入前将清空目标表全部数据"', '"all target-table data will be cleared before import"'),
        ('fmt.Sprintf("有 %d 个源字段无法映射到目标表", len(missingColumns))', 'fmt.Sprintf("%d source columns cannot be mapped to the target table", len(missingColumns))'),
        ('"请选择数据库并提供 SQL 内容"', '"select a database and provide SQL content"'),
        ('"当前数据库为只读模式，禁止执行批量 SQL"', '"the current database is read-only; batch SQL is not allowed"'),
        ('"批量 SQL 写操作必须完成执行前确认"', '"batch SQL writes require confirmation before execution"'),
        ('"事务执行不支持 DDL 或权限语句，请改用顺序执行"', '"transactional execution does not support DDL or privilege statements; use sequential execution"'),
        ('"正在执行 SQL"', '"Executing SQL"'),
        ('fmt.Errorf("第 %d 条 SQL 执行失败: %w", index+1, err)', 'fmt.Errorf("SQL statement %d failed: %w", index+1, err)'),
        ('fmt.Sprintf("已执行 %d/%d 条", index+1, len(statements))', 'fmt.Sprintf("executed %d/%d statements", index+1, len(statements))'),
        ('fmt.Sprintf("执行完成，共 %d 条 SQL", len(statements))', 'fmt.Sprintf("execution completed: %d SQL statements", len(statements))'),
        ('"当前任务没有可下载文件"', '"the current task has no downloadable file"'),
        ('"当前数据库类型不支持 SQL 工作台，可在左侧查看数据库结构与连接状态"', '"the current database type does not support SQL Workbench; review schema and connection status in the sidebar"'),
        ('"筛选字段不存在"', '"filter column does not exist"'),
        ('"系统操作"', '"System Operation"'),
        ('"正在读取表结构"', '"Reading table schema"'),
        ('"导出完成"', '"Export completed"'),
        ('"准备导入任务"', '"Preparing import task"'),
        ('"正在比对源表和目标表结构"', '"Comparing source and target table schemas"'),
        ('"导入完成"', '"Import completed"'),
    ],
    "backend/service/k8s.go": [
        ('const k8sClusterConnectError = "集群连接失败，请检查 kubeconfig"', 'const k8sClusterConnectError = "cluster connection failed; verify kubeconfig"'),
        ('"K8s 集群名称已存在"', '"Kubernetes cluster name already exists"'),
        ('"新增 K8s 集群"', '"Create Kubernetes Cluster"'),
        ('"更新 K8s 集群配置并校验连接"', '"Update Kubernetes Cluster Configuration and Validate Connection"'),
        ('"删除 K8s 集群"', '"Delete Kubernetes Cluster"'),
        ('"解析 kubeconfig 失败: %w"', '"failed to parse kubeconfig: %w"'),
        ('// 集群详情必须复用与集群连接方式一致的客户端。对于 gateway 模式，\n\t// 该客户端会先连接网关，再由网关访问私网 Kubernetes API；不能在\n\t// Ops Admin 所在机器上直接拨号 kubeconfig 中的私网地址。', '// Cluster details must use a client that matches the configured connection mode.\n\t// In gateway mode, the client connects to the gateway before accessing the private Kubernetes API;\n\t// the Ops Admin host must not dial the private kubeconfig address directly.'),
        ('"创建 Kubernetes API 客户端失败: %w"', '"failed to create Kubernetes API client: %w"'),
        ('route := "直连"', 'route := "direct connection"'),
        ('route = "网关转发"', 'route = "gateway relay"'),
        ('"通过%s获取 Kubernetes 集群详情失败: %w"', '"failed to retrieve Kubernetes cluster details through %s: %w"'),
        ('"节点标签键不能为空"', '"node label key is required"'),
        ('"监控数据源不存在"', '"monitoring datasource does not exist"'),
        ('"请选择已启用的 Prometheus 或 VictoriaMetrics 数据源"', '"select an enabled Prometheus or VictoriaMetrics datasource"'),
        ('tls := "未启用"', 'tls := "Disabled"'),
        ('tls = "已启用"', 'tls = "Enabled"'),
        ('// PVC 列表与详情应展示用户声明的申请容量；绑定后 status.capacity\n\t\t// 表示实际绑定 PV 的容量，可能大于 PVC 请求容量。', '// PVC lists and details should display the requested capacity declared by the user; after binding, status.capacity\n\t\t// represents the actual PV capacity and may be larger than the PVC request.'),
        ('Namespace:      "集群级"', 'Namespace:      "Cluster-scoped"'),
        ('{Label: "集群状态", Value: cluster.StatusText}', '{Label: "Cluster Status", Value: cluster.StatusText}'),
        ('{Label: "集群版本", Value: fallbackText(cluster.Version)}', '{Label: "Cluster Version", Value: fallbackText(cluster.Version)}'),
        ('{Label: "节点数量", Value: intLabel(cluster.NodeCount, " 个")}', '{Label: "Node Count", Value: intLabel(cluster.NodeCount, " nodes")}'),
        ('{Label: "Service IP 段", Value: serviceCIDR}', '{Label: "Service CIDR", Value: serviceCIDR}'),
        ('{Label: "容器网络", Value: podCIDR}', '{Label: "Pod Network", Value: podCIDR}'),
        ('serviceCIDR, podCIDR := "未识别", "未识别"', 'serviceCIDR, podCIDR := "Unknown", "Unknown"'),
        ('if podCIDR == "未识别"', 'if podCIDR == "Unknown"'),
        ('parseOverviewCertificate("CA 证书"', 'parseOverviewCertificate("CA Certificate"'),
        ('parseOverviewCertificate("客户端证书"', 'parseOverviewCertificate("Client Certificate"'),
        ('return "expired", "已过期"', 'return "expired", "Expired"'),
        ('return "warning", "即将到期"', 'return "warning", "Expiring Soon"'),
        ('return "valid", "有效"', 'return "valid", "Valid"'),
        ('// 部分受限集群不会返回 ownerReferences；在这种情况下按 Kubernetes\n\t// 控制器生成的 Pod 名称前缀兜底，并优先选择最长的工作负载名称。', '// Some restricted clusters do not return ownerReferences. In that case, fall back to Kubernetes\n\t// controller-generated Pod name prefixes and prefer the longest matching workload name.'),
        ('// 页面列表展示的是 Deployment / StatefulSet 等 Kubernetes Kind，\n\t// 而 API 调用也可能传入小写形式；路径选择统一按规范化的小写值处理。', '// The UI lists Kubernetes kinds such as Deployment and StatefulSet, while API calls may use lowercase values.\n\t// Normalize to lowercase before selecting the resource path.'),
        ('return "由 Kubernetes 引用提供"', 'return "Provided by Kubernetes reference"'),
        ('return fmt.Sprintf("%d核", value/1000)', 'return fmt.Sprintf("%d cores", value/1000)'),
        ('return "集群级"', 'return "Cluster-scoped"'),
        ('// 申请容量优先于绑定 PV 的实际容量，避免把 PV 容量误展示为 PVC 容量。', '// Prefer requested capacity over the bound PV capacity to avoid displaying PV capacity as the PVC request.'),
        ('"闆嗙兢鍚嶇О涓嶈兘涓虹┖"', '"cluster name is required"'),
        ('"kubeconfig 涓嶈兘涓虹┖"', '"kubeconfig is required"'),
        ('"请选择所属环境"', '"select an environment"'),
        ('"集群配置解析失败: %w"', '"failed to parse cluster configuration: %w"'),
        ('"Kubernetes 客户端初始化失败: %w"', '"failed to initialize Kubernetes client: %w"'),
        ('"通过网关连接 API Server 失败（%s）: %w"', '"failed to connect to API Server through gateway (%s): %w"'),
        ('"连接 API Server 失败（%s）: %w"', '"failed to connect to API Server (%s): %w"'),
        ('"连接成功，但读取节点列表失败（请检查 nodes/list 权限）: %w"', '"connection succeeded but node listing failed; verify nodes/list permission: %w"'),
        ('"Pod 瀛樺湪涓嶅彲鍙樺瓧娈碉紝Kubernetes 涓嶅厑璁哥洿鎺ユ洿鏂拌繖閮ㄥ垎鍐呭銆傚缓璁慨鏀瑰彲鍙樺瓧娈碉紝鎴栧垹闄ゅ悗閲嶆柊鍒涘缓 Pod"', '"the Pod contains immutable fields that Kubernetes cannot update directly; modify only mutable fields or recreate the Pod"'),
        ('"褰撳墠瀛樺偍璧勬簮鍖呭惈涓嶅彲鍙樺瓧娈碉紝Kubernetes 涓嶅厑璁哥洿鎺ヨ鐩栦繚瀛樸€傝浠呬慨鏀瑰彲鍙樺瓧娈碉紝鎴栨寜瀛樺偍鍙樻洿娴佺▼澶勭悊"', '"the storage resource contains immutable fields and cannot be overwritten directly; modify only mutable fields or use the storage-change workflow"'),
        ('"褰撳墠璧勬簮鍖呭惈涓嶅彲鍙樺瓧娈碉紝Kubernetes 涓嶅厑璁哥洿鎺ヨ鐩栦繚瀛樸€傝妫€鏌?metadata銆乻elector銆乿olume 绛夊瓧娈垫槸鍚﹁淇敼"', '"the resource contains immutable fields and cannot be overwritten directly; check whether metadata, selector, or volume fields were changed"'),
        ('"YAML 涓殑璧勬簮鏍囪瘑涓庡綋鍓嶉泦缇ょ幇鏈夎祫婧愬啿绐侊紝璇锋鏌ュ悕绉般€佸懡鍚嶇┖闂存垨鍏宠仈瀵硅薄"', '"the YAML resource identity conflicts with an existing cluster resource; verify the name, namespace, and related objects"'),
        ('"目标资源不存在，可能已被删除或命名空间已变化，请刷新后重试"', '"target resource does not exist; it may have been deleted or moved to another namespace; refresh and retry"'),
        ('"YAML 鏍￠獙鏈€氳繃锛岃妫€鏌ュ瓧娈垫牸寮忋€乤piVersion銆乲ind 浠ュ強 spec 鍐呭鏄惁姝ｇ‘"', '"YAML validation failed; verify field formats, apiVersion, kind, and spec content"'),
        ('return "部分告警"', 'return "Partial Alerts"'),
        ('return "离线"', 'return "Offline"'),
        ('return "运行中"', 'return "Running"'),
        ('return "刚刚"', 'return "Just now"'),
    ],
}

HAN = re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF]")


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    changed_files = 0
    replacement_count = 0
    failures: list[str] = []
    for relative_path, replacements in REPLACEMENTS.items():
        path = root / relative_path
        original = path.read_text(encoding="utf-8")
        updated = original
        for old, new in replacements:
            count = updated.count(old)
            if count:
                updated = updated.replace(old, new)
                replacement_count += count
        if updated != original:
            path.write_text(updated, encoding="utf-8")
            changed_files += 1
        remaining = [(n, line) for n, line in enumerate(updated.splitlines(), 1) if HAN.search(line)]
        if remaining:
            preview = "; ".join(f"{n}: {line.strip()}" for n, line in remaining[:12])
            failures.append(f"{relative_path} still contains Han characters: {preview}")
    print(f"batch5 localized {replacement_count} occurrence(s) across {changed_files} file(s)")
    if failures:
        for failure in failures:
            print(f"ERROR: {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
