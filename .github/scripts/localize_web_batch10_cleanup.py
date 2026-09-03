#!/usr/bin/env python3
from pathlib import Path
import re
import sys

path = Path(__file__).resolve().parents[2] / "web/src/views/applications/AppPipelineCenter.vue"
text = path.read_text(encoding="utf-8")
replacements = [
    ("执行编译、打包等Build Command", "Compile, Package 등 Build Command 실행"),
    ("配置현재 Stage", "현재 Stage 구성"),
    ("선택하십시오: 镜像仓库", "Image Registry를 선택하십시오."),
    ("资产Host #", "Asset Host #"),
    ("입력하십시오: 第 ${index + 1} 个Stage 이름", "${index + 1}번째 Stage 이름을 입력하십시오."),
    ("입력하십시오: 「${stage.name}」的执行Command", "${stage.name} Stage의 실행 Command를 입력하십시오."),
    ("선택하십시오: 「${stage.name}」使用的Notification Rule", "${stage.name} Stage에서 사용할 Notification Rule을 선택하십시오."),
    ("人工审批${action}", "Manual Approval ${action}"),
    (">历史<", ">History<"),
    (" 个Stage", "개 Stage"),
    ("执行记录", "Run History"),
    ("Pipeline / Application / 镜像 Tag", "Pipeline / Application / Image Tag"),
    ("执行编号", "Run ID"),
    ("镜像 Tag", "Image Tag"),
    ("开始时间", "Start Time"),
    ("详情/Log", "상세 / Log"),
    ("选择Pipeline Template", "Pipeline Template 선택"),
    ("从空白画布开始，사용자 정의所有Stage。", "빈 Canvas에서 시작해 모든 Stage를 사용자 정의합니다."),
    ("使用选中模板", "선택한 Template 사용"),
    ("所属Application", "Application"),
    ("默认Branch", "Default Branch"),
    ("默认使用ApplicationBranch", "기본값은 Application Branch"),
    ("默认Environment", "Default Environment"),
    ("선택하십시오: 执行代码、Build、镜像与发布的主机", "Source, Build, Image, Deploy를 실행할 Host를 선택하십시오."),
    ("PipelineStage会승인 SSH 在所选资产主机上执行；请确保该主机配置认证凭据，并安装 Git/SVN、Docker、kubectl 等所需工具。", "Pipeline Stage는 선택한 Asset Host에서 SSH로 실행됩니다. 해당 Host에 인증 Credential을 구성하고 Git/SVN, Docker, kubectl 등 필요한 Tool을 설치하십시오."),
    ("label=\"描述\"", "label=\"설명\""),
    ("说明Pipeline用途、Environment和风险提示", "Pipeline 용도, Environment 및 Risk를 설명하십시오."),
    ("Stage编排", "Stage Orchestration"),
    ("현재为빈 Pipeline，请添加Stage", "현재 빈 Pipeline입니다. Stage를 추가하십시오."),
    ("上移Stage", "Stage 위로 이동"),
    ("下移Stage", "Stage 아래로 이동"),
    ("예：BuildApplication镜像", "예: Application Image Build"),
    ("<Application编码>", "<Application Code>"),
    ("{时间}", "{Time}"),
    ("K8s 集群", "Kubernetes Cluster"),
    ("选择 K8s 集群", "Kubernetes Cluster 선택"),
    (">刷新<", ">새로고침<"),
]
for old, new in replacements:
    text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
remaining = [(n, line) for n, line in enumerate(text.splitlines(), 1) if re.search(r'[\u3400-\u4DBF\u4E00-\u9FFF]', line)]
print(f"web batch10 cleanup left {len(remaining)} Han line(s)")
for n, line in remaining[:50]:
    print(f"REMAINING: {n}: {line.strip()}", file=sys.stderr)
raise SystemExit(1 if remaining else 0)
