package opdef

import "net/http"

// monitorDefs covers the /monitor/* and /k8s/* section of the authGroup. The
// datasource reads are sensitive (the datasource payload struct carries
// password and token without json:"-"), the k8s cluster reads return the
// kubeConfig plaintext, secret/configmap details and pod logs expose cluster
// content, so they are curated into the table with redaction metadata.
// /k8s/* reuses the assets:k8s:* vocabulary (plan §2.4 rule 4); monitor query
// and log execution reuse the seeded page-menu values monitor:query and
// monitor:logs.
var monitorDefs = []Def{
	{Method: http.MethodGet, Path: "/monitor/datasource/list", Permission: "monitor:datasource:list", Risk: RiskHigh,
		Redaction: []string{"datasource.password", "datasource.token"}},
	{Method: http.MethodGet, Path: "/monitor/datasource/options", Permission: "monitor:datasource:list", Risk: RiskHigh,
		Redaction: []string{"datasource.password", "datasource.token"}},
	{Method: http.MethodGet, Path: "/monitor/datasource/info", Permission: "monitor:datasource:list", Risk: RiskHigh,
		Redaction: []string{"datasource.password", "datasource.token"}},
	{Method: http.MethodPost, Path: "/monitor/datasource/save", CreateEdit: [2]string{"monitor:datasource:add", "monitor:datasource:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/datasource/delete", Permission: "monitor:datasource:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/datasource/test", Permission: "monitor:datasource:test", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/query/instant", Permission: "monitor:query", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/query/range", Permission: "monitor:query", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/logs/query", Permission: "monitor:logs", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/logs/shortcuts/save", Permission: "monitor:logs", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/logs/shortcuts/delete", Permission: "monitor:logs", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/traces/query", Permission: "monitor:traces", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/alert/template/group/save", CreateEdit: [2]string{"monitor:alerttemplate:add", "monitor:alerttemplate:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/alert/template/group/delete", Permission: "monitor:alerttemplate:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/alert/template/save", CreateEdit: [2]string{"monitor:alerttemplate:add", "monitor:alerttemplate:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/alert/template/delete", Permission: "monitor:alerttemplate:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/alert/template/prometheus/parse", Permission: "monitor:alerttemplate:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/template/prometheus/import", Permission: "monitor:alerttemplate:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/template/prometheus/export", Permission: "monitor:alerttemplate:edit", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/alert/rule/save", CreateEdit: [2]string{"monitor:alertrule:add", "monitor:alertrule:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/alert/rule/delete", Permission: "monitor:alertrule:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/monitor/alert/rule/status", Permission: "monitor:alertrule:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/rule/batch", Permission: "monitor:alertrule:batch", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/rule/run", Permission: "monitor:alertrule:run", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/rule/preview", Permission: "monitor:alertrule:edit", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/silence/rule/save", CreateEdit: [2]string{"monitor:silence:add", "monitor:silence:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/silence/rule/preview", Permission: "monitor:silence:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/silence/rule/delete", Permission: "monitor:silence:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/silence/rule/batch", Permission: "monitor:silence:edit", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/aggregation/rule/save", CreateEdit: [2]string{"monitor:aggregation:add", "monitor:aggregation:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/aggregation/rule/delete", Permission: "monitor:aggregation:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/aggregation/rule/batch", Permission: "monitor:aggregation:edit", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/alert/event/claim", Permission: "monitor:alertevent:claim", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/event/resolve", Permission: "monitor:alertevent:close", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/event/batch", Permission: "monitor:alertevent:close", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/monitor/alert/event/action", Permission: "monitor:alertevent:action", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/monitor/dashboard/save", CreateEdit: [2]string{"monitor:dashboard:add", "monitor:dashboard:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/dashboard/delete", Permission: "monitor:dashboard:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/dashboard/panel/save", CreateEdit: [2]string{"monitor:dashboard:add", "monitor:dashboard:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/monitor/dashboard/panel/delete", Permission: "monitor:dashboard:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/monitor/dashboard/panel/query", Permission: "monitor:query", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodGet, Path: "/k8s/cluster/list", Permission: "assets:k8s:cluster", Risk: RiskHigh,
		Redaction: []string{"cluster.kubeConfig"}},
	{Method: http.MethodGet, Path: "/k8s/cluster/info", Permission: "assets:k8s:cluster", Risk: RiskHigh,
		Redaction: []string{"cluster.kubeConfig"}},
	{Method: http.MethodPost, Path: "/k8s/cluster/add", Permission: "assets:k8s:cluster:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/k8s/cluster/update", Permission: "assets:k8s:cluster:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/k8s/cluster/delete", Permission: "assets:k8s:cluster:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodGet, Path: "/k8s/cluster/detail", Permission: "assets:k8s:cluster", Risk: RiskHigh,
		Redaction: []string{"cluster.kubeConfig"}},
	{Method: http.MethodPut, Path: "/k8s/node/labels", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/k8s/service/update", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/k8s/configmap/detail", Permission: "assets:k8s:configstorage", Risk: RiskLow,
		Redaction: []string{"configmap.data"}},
	{Method: http.MethodGet, Path: "/k8s/secret/detail", Permission: "assets:k8s:configstorage", Risk: RiskHigh,
		Redaction: []string{"secret.data"}},
	{Method: http.MethodGet, Path: "/k8s/pod/logs", Permission: "assets:k8s:pod", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/k8s/workload/scale", Permission: "assets:k8s:workload:scale", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/k8s/workload/restart", Permission: "assets:k8s:workload:restart", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/k8s/workload/images", Permission: "assets:k8s:workload:image", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/k8s/workload/resources", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/k8s/istio/traffic", Permission: "assets:k8s:advancednetwork", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/k8s/httproute/traffic", Permission: "assets:k8s:advancednetwork", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/k8s/resource/yaml/create", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/k8s/resource/yaml", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/k8s/resource/delete", Permission: "assets:k8s:resource:delete", Mutating: true, Risk: RiskHigh},
}
