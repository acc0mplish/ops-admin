package opdef

import "net/http"

// opsDefs covers the /ops/*, /notify/* section of the authGroup. Seeded
// ops:* and notify:* button vocabulary is reused wherever it matches; the
// ops:application:* group (build tasks, releases, pipelines, image
// registries) has no seeded vocabulary and is new. Shared /save endpoints
// follow the domains precedent with CreateEdit pairs (plan §2.4 rule 2).
// The notify channel reads are sensitive: mapNotifyChannel returns the
// channel secret in plaintext.
var opsDefs = []Def{
	{Method: http.MethodPost, Path: "/ops/script/add", Permission: "ops:script:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/script/update", Permission: "ops:script:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/script/status", Permission: "ops:script:status", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/script/rollback", Permission: "ops:script:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/script/delete", Permission: "ops:script:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/ops/exec/command", Permission: "ops:quickexec:command:execute", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/exec/script", Permission: "ops:quickexec:script:execute", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/exec/file-dispatch", Permission: "ops:quickexec:file:execute", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/exec/retry", AnyOf: []string{"ops:quickexec:command:execute", "ops:quickexec:script:execute", "ops:quickexec:file:execute"}, Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/ops/schedule/task/add", Permission: "ops:schedule:task:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/schedule/task/update", Permission: "ops:schedule:task:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/schedule/task/status", Permission: "ops:schedule:task:status", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/schedule/task/run", Permission: "ops:schedule:task:run", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/ops/schedule/task/delete", Permission: "ops:schedule:task:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/ops/schedule/task/batch/delete", Permission: "ops:schedule:task:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/schedule/template/add", Permission: "ops:schedule:template:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/schedule/template/update", Permission: "ops:schedule:template:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/schedule/template/delete", Permission: "ops:schedule:template:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/ops/job/add", Permission: "ops:job:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/job/update", Permission: "ops:job:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/job/run", Permission: "ops:job:execute", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/ops/job/status", Permission: "ops:job:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/job/delete", Permission: "ops:job:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/job/history/approve", Permission: "ops:job:approve", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/job/history/reject", Permission: "ops:job:approve", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/job/template/add", Permission: "ops:job:template:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/job/template/update", Permission: "ops:job:template:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/job/template/status", Permission: "ops:job:template:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/job/template/delete", Permission: "ops:job:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/ops/environment/save", CreateEdit: [2]string{"ops:environment:add", "ops:environment:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/environment/delete", Permission: "ops:environment:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/ops/application/save", CreateEdit: [2]string{"ops:application:add", "ops:application:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/application/delete", Permission: "ops:application:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/build-task/save", CreateEdit: [2]string{"ops:application:buildtask:add", "ops:application:buildtask:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/application/build-task/status", Permission: "ops:application:buildtask:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/application/build-task/delete", Permission: "ops:application:buildtask:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/build-task/run", Permission: "ops:application:buildtask:run", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/release/run", Permission: "ops:application:release:run", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/release/retry", Permission: "ops:application:release:run", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/image-registry/save", CreateEdit: [2]string{"ops:application:registry:add", "ops:application:registry:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/application/image-registry/delete", Permission: "ops:application:registry:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/pipeline/save", CreateEdit: [2]string{"ops:application:pipeline:add", "ops:application:pipeline:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/ops/application/pipeline/status", Permission: "ops:application:pipeline:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/ops/application/pipeline/delete", Permission: "ops:application:pipeline:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/pipeline/copy", Permission: "ops:application:pipeline:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/application/pipeline/run", Permission: "ops:application:pipeline:run", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/ops/application/pipeline/run/approve", Permission: "ops:application:pipeline:approve", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/ops/application/pipeline/run/rollback", Permission: "ops:application:pipeline:run", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodGet, Path: "/notify/channel/list", Permission: "notify:channel:list", Risk: RiskHigh,
		Redaction: []string{"channel.secret"}},
	{Method: http.MethodGet, Path: "/notify/channel/options", Permission: "notify:channel:list", Risk: RiskHigh,
		Redaction: []string{"channel.secret"}},
	{Method: http.MethodGet, Path: "/notify/channel/info", Permission: "notify:channel:list", Risk: RiskHigh,
		Redaction: []string{"channel.secret"}},
	{Method: http.MethodPost, Path: "/notify/channel/save", CreateEdit: [2]string{"notify:channel:add", "notify:channel:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/notify/channel/delete", Permission: "notify:channel:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/notify/template/save", CreateEdit: [2]string{"notify:template:add", "notify:template:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/notify/template/delete", Permission: "notify:template:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/notify/rule/save", CreateEdit: [2]string{"notify:rule:add", "notify:rule:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/notify/rule/test", Permission: "notify:rule:test", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/notify/rule/delete", Permission: "notify:rule:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/notify/send-log/retry", Permission: "notify:sendlog:retry", Mutating: true, Risk: RiskMedium},
}
