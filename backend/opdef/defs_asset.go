package opdef

import "net/http"

// assetDefs covers the /asset/* and /dbms/* section of the authGroup. Every
// mutation is defined; the sensitive GET reads are the ones verified in the
// curation table of the plan: asset host list/info (which return the
// preloaded credential and cloud account secrets unmasked), the credential
// reads, the cloud account list/info (secretKey field) and the database dump
// downloads. Gateway list/info are normal reads — the service masks the
// credential fields, and database list/info blank the password column.
// /dbms/* reuses the assets:database:* vocabulary (plan §2.4 rule 4).
var assetDefs = []Def{
	{Method: http.MethodGet, Path: "/asset/host/list", Permission: "assets:host:list", Risk: RiskHigh,
		Redaction: []string{"host.credential.password", "host.credential.privateKey", "host.credential.passphrase", "host.cloudAccount.secretKey"}},
	{Method: http.MethodGet, Path: "/asset/host/info", Permission: "assets:host:list", Risk: RiskHigh,
		Redaction: []string{"host.credential.password", "host.credential.privateKey", "host.credential.passphrase", "host.cloudAccount.secretKey"}},
	{Method: http.MethodPost, Path: "/asset/host/add", Permission: "assets:host:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/asset/host/import", Permission: "assets:host:import", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/asset/host/cloudSync", Permission: "assets:cloudaccount:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/host/update", Permission: "assets:host:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/asset/host/sync", Permission: "assets:host:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/asset/host/batch/sync", Permission: "assets:host:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/host/batch/credential", Permission: "assets:host:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/host/group/remove", Permission: "assets:hostgroup:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/host/delete", Permission: "assets:host:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/asset/host/batch/delete", Permission: "assets:host:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/asset/hostGroup/add", Permission: "assets:hostgroup:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/hostGroup/update", Permission: "assets:hostgroup:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/hostGroup/delete", Permission: "assets:hostgroup:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodGet, Path: "/asset/credential/list", Permission: "assets:credential:list", Risk: RiskHigh,
		Redaction: []string{"credential.password", "credential.privateKey", "credential.passphrase"}},
	{Method: http.MethodGet, Path: "/asset/credential/options", Permission: "assets:credential:list", Risk: RiskHigh,
		Redaction: []string{"credential.password", "credential.privateKey", "credential.passphrase"}},
	{Method: http.MethodGet, Path: "/asset/credential/info", Permission: "assets:credential:list", Risk: RiskHigh,
		Redaction: []string{"credential.password", "credential.privateKey", "credential.passphrase"}},
	{Method: http.MethodPost, Path: "/asset/credential/add", Permission: "assets:credential:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/credential/update", Permission: "assets:credential:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/credential/delete", Permission: "assets:credential:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodGet, Path: "/asset/cloudAccount/list", Permission: "assets:cloudaccount:list", Risk: RiskHigh,
		Redaction: []string{"cloudAccount.accessKey", "cloudAccount.secretKey"}},
	{Method: http.MethodGet, Path: "/asset/cloudAccount/info", Permission: "assets:cloudaccount:list", Risk: RiskHigh,
		Redaction: []string{"cloudAccount.accessKey", "cloudAccount.secretKey"}},
	{Method: http.MethodPost, Path: "/asset/cloudAccount/add", Permission: "assets:cloudaccount:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/cloudAccount/update", Permission: "assets:cloudaccount:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/cloudAccount/delete", Permission: "assets:cloudaccount:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/asset/gateway/add", Permission: "assets:gateway:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/gateway/update", Permission: "assets:gateway:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/gateway/delete", Permission: "assets:gateway:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/asset/gateway/status", Permission: "assets:gateway:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/asset/gateway/test", Permission: "assets:gateway:test", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/asset/database/add", Permission: "assets:database:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/asset/database/update", Permission: "assets:database:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/database/delete", Permission: "assets:database:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/asset/database/test", Permission: "assets:database:test", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/asset/service/save", Permission: "assets:service:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/asset/service/delete", Permission: "assets:service:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/asset/service/workload/rollback", Permission: "assets:service:rollback", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/asset/service/workload/logs", Permission: "assets:service:logs", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/asset/service/diagnosis/processes", Permission: "assets:service:diagnosis", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/asset/service/diagnosis/environment", Permission: "assets:service:diagnosis", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/asset/service/diagnosis/run", Permission: "assets:service:diagnosis", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/asset/service/diagnosis/arthas/download", Permission: "assets:service:arthas", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/asset/service/diagnosis/arthas/upload", Permission: "assets:service:arthas", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/dbms/schema/create", Permission: "assets:database:data:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/sql/execute", Permission: "assets:database:sql:execute", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/dbms/sql/analyze", Permission: "assets:database:sql:execute", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/redis/analyze", Permission: "assets:database:sql:execute", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/redis/execute", Permission: "assets:database:sql:execute", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/dbms/table/row/insert", Permission: "assets:database:data:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/dbms/table/row/update", Permission: "assets:database:data:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/dbms/table/row/delete", Permission: "assets:database:data:edit", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/dbms/task/export", Permission: "assets:database:export", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/task/import", Permission: "assets:database:import:execute", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/task/import/precheck", Permission: "assets:database:import:execute", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/task/batch-sql", Permission: "assets:database:sql:execute", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodGet, Path: "/dbms/task/download", Permission: "assets:database:export", Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/dbms/backup/plan/save", Permission: "assets:database:backup:create", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/dbms/backup/plan/delete", Permission: "assets:database:backup:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/dbms/backup/plan/run", Permission: "assets:database:backup:create", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/dbms/backup/manual", Permission: "assets:database:backup:create", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/dbms/backup/download", Permission: "assets:database:export", Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/dbms/backup/import", Permission: "assets:database:backup:restore", Mutating: true, Risk: RiskMedium},
}
