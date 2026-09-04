# Cloud 비용 FinOps 최적화 방안

업데이트 시각: 2026-07-17  
범위: `backend/*finops*`, `web/src/views/integration/finops/*`, 관련 API. 본 문서는 무관한 Module을 다루지 않습니다.

## 1. 현황 평가

| Capability | 현재 구현 | 결론 |
| --- | --- | --- |
| Cloud Account | AWS, Azure, GCP, Alibaba Cloud, Tencent Cloud, Custom Account를 관리할 수 있음. Credential은 API 응답에 미포함 | Alibaba Cloud, Tencent Cloud는 내장 Billing Fetch 보유. AWS/Azure/GCP는 현재 Custom Billing Adapter Interface로 연계되므로 내장 공식 동기화로 잘못 표기해서는 안 됨 |
| Billing 동기화 | 기본으로 현재 Calendar Month를 포함한 최근 6개 Calendar Month를 동기화. 월 단위 실행, 실패 격리, 외부 ID 기준 upsert, 월별 실행 Log 보존 | 핵심 동기화 규칙은 충족. Page에서 Billing Month 범위를 선택하게 하고, 월간 총 Billing을 일 단위로 분할하는 추정 기준을 명확히 해야 함 |
| 비용 Dashboard | 3/6/12개월 Trend, Cloud Provider 분포, 누적/이번 달/지난달/전월 대비/최근 동기화 시각 표시 가능 | Cloud Account Filter, Billing Data 품질 안내, 통합 Backend 집계 Metric 부재 |
| 비용 분할 | 단일 Calendar Month, Product/Region 및 비용 상세 기준 | Backend는 Cloud Account·Label·Resource 차원을 갖추었으나 Page에 완전히 노출되지 않음. 상세에는 "정확 일별 Billing/추정 일별 Billing"을 표기해야 함 |
| Resource 분할 | Cloud Account와 날짜 필수 선택. Region·Resource Type Filter 지원 | 집계 Key가 Resource ID만 사용하므로 서로 다른 Cloud Account의 동일 Resource ID가 병합될 위험 존재 |
| 최적화 권고 | 기본 Strategy, AI Model 선택, HTML Report, PDF 인쇄, 삭제 지원 | 기본/AI Strategy가 유휴 Resource·저이용률·과금 방식·유휴 기반 Resource·절감액을 이미 포괄. Cloud Account/Billing Month 분석 범위가 없고, Runtime Metric이 없으면 "확인 필요"로만 표기할 수 있으며 추론을 사실로 삼아서는 안 됨 |
| 동기화 History | 동기화 상태, Record 수, 금액, Billing Month, 소요 시간, 오류 정보 보유 | Frontend·Backend Field 불일치는 이미 수정됨. 동기화 결과를 Trigger 후 월 단위로 볼 수 있게 해야 함 |

## 2. Data 기준과 경계

1. "최근 6개월"은 항상 "현재 Calendar Month를 포함한 최근 6개 Calendar Month"를 의미합니다.
2. 동기화는 Calendar Month 단위로 월별 실행되며, 한 달의 실패가 다른 달에 영향을 주지 않습니다.
3. 현재 달은 불완전할 수 있으므로 모든 Page에 "현재 시점 기준"을 명확히 표시해야 합니다.
4. Alibaba Cloud는 현재 월 단위 Instance Billing을 한 번에 Fetch한 뒤 로컬에서 해당 달의 유효 일수로 일 단위 통계를 분할하여, 매일 Cloud API를 호출해 Timeout이 발생하는 것을 피합니다. 이러한 Record는 반드시 `daily_estimate` 식별자로 표시하며 Cloud Provider의 정확 일별 Billing과 혼동해서는 안 됩니다.
5. Resource Runtime Metric(CPU, Memory, IOPS, Connection 수)이 없을 때 최적화 권고는 확인 권고일 수만 있으며, Resource가 유휴이거나 저이용률이라고 단정해서는 안 됩니다.

## 3. 실행 계획

### P0: 이번 구현

1. **Cloud Account Capability 투명화**
   - API가 Billing 연계 Capability를 반환: Alibaba Cloud/Tencent Cloud는 내장 API, AWS/Azure/GCP/Custom은 Adapter 방식.
   - Page에 Capability와 Config 안내를 명확히 표시해 "모든 Cloud Provider의 동기화가 내장되어 있다"는 오해를 방지합니다.

2. **Dashboard·분할 기준 완성**
   - Dashboard가 Cloud Account Filter를 지원하고, Backend가 이번 달·지난달·추정 일별 Billing 건수·최근 동기화 시각 등 Metric을 한 번에 반환합니다.
   - 비용 분할은 Product·Region·일별 상세를 유지하고, 고급 차원(Cloud Account·Label·Resource)을 추가하며 Data 품질 식별자를 표시합니다.

3. **Resource 분할 정확성**
   - `Cloud Account + Resource ID/이름 + Resource Type`을 집계 Key로 사용해 Cloud Account 간 잘못된 병합을 차단합니다.
   - Cloud Account·Date Range 필수 선택 및 Region·Resource Type 다중 선택 Filter를 유지합니다.

4. **동기화 제어 가능성과 가시성**
   - 동기화 Page에 "최근 3/6/12개월" 범위 선택을 제공하고 이번 월별 결과를 표시합니다.
   - 실행 History에 Cloud Account·Provider·Trigger 방식·Billing Month·Record 수·금액·소요 시간·시작/완료 시각·오류 원인을 표시합니다.

5. **권고 생성 범위**
   - 권고 생성 시 Cloud Account와 분석 Billing Month를 선택할 수 있게 하며, 기본 Strategy와 AI Strategy가 동일한 FinOps 분류 Framework를 사용합니다.
   - Report에 분석 범위와 "Billing 추론/확인 필요" 설명을 표시합니다.

### P1: 후속 Iteration

1. AWS Cost Explorer, Azure Cost Management, GCP Billing Export의 공식 Adapter를 연계하고 IAM/Service Principal/OAuth 승인 Model로 각각 모델링합니다.
2. Resource 모니터링 Data를 연계해 저이용률·유휴 Resource 권고에 검증 가능한 근거(Time Window·Threshold·Metric 값)를 추가합니다.
3. Budget, Anomaly Detection, 비용 귀속 규칙(Business Unit/Project/Environment)과 Budget Alert Loop를 도입합니다.
4. Cloud Credential에 KMS/Envelop Encryption, Rotation 기록, 최소 권한 검사를 적용합니다. 현재 Database의 Credential Field는 완성된 암호화 방안으로 간주해서는 안 됩니다.

## 4. API 계약

| API | 이번 계약 |
| --- | --- |
| `GET /api/v1/integration/finops/dashboard` | `start`, `end`, `account_id` 지원. 집계 Metric, 월별 Trend, Provider 분포, Data 품질 통계를 반환합니다. |
| `GET /api/v1/integration/finops/breakdown` | `month=YYYY-MM` 필수 전달. `account_id`와 `dimension=service/region/account/tag/resource/detail` 지원. |
| `GET /api/v1/integration/finops/resource/list` | Cloud Account와 시작·종료 날짜 필수. `region`, `resource_type` 다중 선택 지원. |
| `POST /api/v1/integration/finops/sync/trigger` | `accountId` 필수. `start_month`, `end_month` 선택, 생략 시 최근 6개 Calendar Month. |
| `POST /api/v1/integration/finops/recommendation/generate` | `strategy`, `modelId`, `account_id`, `month` 지원. AI 미구성 또는 기본 Strategy 선택 시 내장 Rule 사용. |

## 5. 수용 기준

- 비용 Dashboard는 3/6/12개월과 Cloud Account를 전환할 수 있고, Trend는 0으로 이어져 채워지며, 현재 달에 "현재 시점 기준" 식별자가 표시됩니다.
- 비용 분할은 선택한 Calendar Month만 집계하며 Product·Region·Cloud Account·Label·Resource 및 일별 상세의 기준이 일치합니다.
- 일 단위 분할 Data에는 명확한 "추정" 식별자가 있고 "정확 일별 Billing"을 가장한 표시가 없습니다.
- Resource 분할은 Cloud Account·Time Range가 선택되지 않으면 Data 조회를 시작하지 않고, 서로 다른 Cloud Account의 동명/동일 ID Resource는 병합되지 않습니다.
- 동기화 Page에서 범위를 선택할 수 있고 월별 성공/실패·건수·금액·오류를 반환하며, History Field가 완전합니다.
- 최적화 Report에 분석 Cloud Account·Billing Month·Strategy·Data 제약과 5가지 FinOps Strategy를 표시하고, 권고는 조회·PDF 인쇄·삭제할 수 있습니다.
- Backend Test와 Frontend Build를 통과합니다. 실제 임시 기동으로 Frontend·Backend를 검증한 뒤 자동으로 종료됩니다.
