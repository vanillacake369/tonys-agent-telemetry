# AGENT_LOG — 4-perspective Project Review

**Date:** 2026-05-25
**Branch:** `main` @ `3e77660`
**Scope:** ~16K LOC Go Bubbletea TUI for Claude Code observability
**Method:** 4 background sub-agents in parallel (2 reviewers, 2 researchers)

---

## TL;DR

네 리뷰가 한목소리로 가리키는 결론:

> **"DAG/세션/스킬을 다 보여주는 것"보다 "Claude Code 훅을 가장 안전하고 정확하게 받아내는 local-first control plane"이 되는 것에 본질이 있다. 두 축 모두에서 *계약(contract)·가드레일·다이렉트한 가시화*가 비어 있다.**

| 차원 | 현재 상태 | 가장 큰 결함 1개 |
|---|---|---|
| 1. 규격/PL | 도메인 타입은 있지만 외부 계약(JSONL/Hook)을 reverse-engineering 한 raw struct에 의존 | `event.Event.Payload`가 `json.RawMessage` only, 버전·타입 디스크리미네이터 없음 |
| 2. Swarm 유틸성 | DAG 파서는 80% 구현되어 있는데 **렌더링 탭이 없음** | "사용자가 볼 수 있는 DAG 화면이 없다" — 핵심 가치 미배포 |
| 3. 가드레일 | TUI 골격은 안정, 그러나 OS 경계가 깨지면 침묵 | `signal.Notify` 미사용 + `fifoCancel` 미호출 → 터미널 깨짐 + 고루틴 누수 |
| 4. 동향 | 로컬-퍼스트 + 훅 기반 + DAG 시각화 = 빈 슬롯 | MCP 레지스트리/OTel GenAI semconv 흐름에 아직 미연결 |

**추천 전략:** 하이브리드 — Strategy C (Contract-first) + Strategy A의 P0 가드레일 4건 → Strategy B (Ship DAG). 자세한 비교는 [Part 4](#part-4--trade-off-matrix).

---

# Part 1 — Review Summaries

## 1.1 Spec / PL Principles (Agent 1: Reviewer)

**총평:** 코드 자체는 깔끔하지만, **외부 세계와 맞닿은 계약(contract)이 전부 묵시적**. JSONL과 hook payload라는 두 외부 스키마에 전적으로 의존하면서 둘 다 reverse-engineered raw struct로 처리.

### Critical
| ID | 위치 | 문제 |
|---|---|---|
| C1 | `internal/event/fifo.go:22-24`, `cmd/hook-handler/main.go:38` | Hook payload가 `json.RawMessage`만. 타입드 envelope·버전 디스크리미네이터 없음 |
| C2 | `internal/data/jsonl.go:16-53` | JSONL schema 6개 raw struct가 관찰 기반. `sessionId`→`session_id` 같은 변경 시 silent drop |
| C3 | `internal/skill/recommender.go:121,141,156,164` | RepoURL 4건이 `vanillacake369/awesome-skills/{path}` 형식으로 깨진 GitHub URL |

### Major
- `tab_sessions.go:592-597` — `max()`가 Go 1.21 builtin을 shadowing
- `go.mod` — 모든 require가 `// indirect`. `bubbletea`/`lipgloss`/`bubbles` 직접 의존인데 indirect로 표기
- `provider.go:108-113` — `sortSessionsByTime`이 O(n²) hand-rolled insertion sort, 같은 패키지에서 `sort.Slice` 사용 중
- `internal/tui/tab_hooks.go:23-45` — `HooksConfig` 도메인 스키마가 UI 패키지에 정의됨
- `internal/data/model.go:35` — `DAGNode.Status string`이 sum type이 아님 (4개 valid 값 문서화만)

### DDD/SRP/DRY
- `internal/skill`은 bounded context가 아니라 services 잡탕 (model/infra/application 혼재)
- `internal/data/jsonl.go` — 777줄 god file (7개 parse 함수 + 6개 raw struct + DAG 파싱)
- `bufio.NewScanner + Buffer(1<<20)` 패턴 7회 복붙 (jsonl.go:170,295,339,447,567,596,728)
- `renderSkillListWithSearch`가 `RenderSearchBar` helper 재구현 (`tab_skills.go:648-675` vs `layout.go:50-84`)
- `tab_skills.go:437-441` — debounce가 `time.Sleep` in goroutine (취소 불가, `tea.Tick` 권장)

### Type-encoded invariants
- `App.searchFocused bool` + `detailView *DetailView` + `whichKey.visible bool` 3개 모달 bool → sum type 필요
- `recommender.go:38` — Claude Code 도구가 `~/.gemini/cache/`에 캐시 (명백한 버그)
- `model.go:8` — `Session.Provider` zero-value 가드 없음

### IaC
- `flake.nix:5` — `nixos-unstable` commit pin 없음 (lock이 막아주지만 contributor 혼란)
- `Makefile`에 version 주입 타깃 없음 → `go build`는 `"dev"` 표기
- `go.mod`의 `go 1.26.1` 존재하지 않는 버전 (오타)

---

## 1.2 Swarm/Workflow Product Perspective (Agent 2: Researcher)

### 지금 진짜로 쓸모 있는 부분
- **`internal/event/fifo.go`의 로컬-퍼스트 hook integration** — 실질적 차별점. Langfuse/Helicone과 달리 API round-trip 없이 PreToolUse/PostToolUse 실시간 수신, hook handler가 **절대 블록하지 않음** (항상 exit 0)
- **`tab_sessions.go`의 session resume/fork** — sub-agent 중간 실패 시 fork로 부분 replay. Langfuse엔 없는 ad-hoc multi-agent 핵심
- **DAG 파서는 있으나 화면이 없음** — `ParseDAG`, `DAGNode.Children`, `AgentType`, `Status`까지 데이터 모델 완성. **TUI 시각화 탭 미존재**

### 빠진 것 (impact ↓)
| Priority | 기능 | Impact | Effort |
|---|---|---|---|
| P1 | Cost-per-agent budgeting + kill switch | Critical | Medium |
| P2 | 실시간 causal DAG 시각화 (tab_dag.go) | Critical | High |
| P3 | Cross-session causal trace (parent_session_id) | High | Medium |
| P4 | Pluggable telemetry sinks (OTLP/Honeycomb/Langfuse) | High | Medium |
| P5 | Replay/time-travel | Medium | High |
| P6 | Permission/tool-use audit trail | Medium | Low |

### 포지셔닝
**유일한 niche:** *local-first, TUI-native, hook-integrated, swarm-aware*. 클라우드(Langfuse/Phoenix), 파일 파서(ccusage), GUI(claudia) 모두 동시 충족 못함.
**잃는 영역:** Lambda/K8s cloud swarm 관측 (Phoenix), eval rigor (Phoenix), proxy 기반 cost (Helicone).

### 로드맵 권고
- **Phase 1 (4주):** `tab_dag.go` 출시, cross-session breadcrumb, README "Swarm Orchestration Debugging" 섹션
- **Phase 2 (8주):** per-agent cost budget + PreToolUse guard, `internal/sink/` 인터페이스 stub
- **Phase 3 (3-6개월):** OTLP/Honeycomb sink 실구현, `internal/eval/`, replay/sandbox
- **잘라낼 것:** `recommender.go` (DAG로 인력 재배치), `tab_agents` (Hooks 탭에 흡수)

---

## 1.3 Guardrails & IPC Audit (Agent 3: Reviewer)

**Critical: 0건** — 데이터 손상/RCE급 결함 없음.

### Major (5건)
| ID | 위치 | 시나리오 | 수정 1줄 |
|---|---|---|---|
| M1 | `main.go` 전체 | SIGTERM/SIGHUP에 raw kill → alt-screen 잔존, `reset` 필요 | `signal.Notify(SIGTERM, SIGHUP)` → `p.Quit()` |
| M2 | `main.go:46` | `tonys-agent-telemetry \| tee log` 시 ioctl 에러 후 종료 | `term.IsTerminal(stdout.Fd())` 사전 체크 |
| M3 | `tab_sessions.go:165,175` + `terminal.go:36` | multiplexer 없을 때 `bash -c` fallback이 TUI와 tty 동시 점유 → 화면 깨짐 | `tea.ExecProcess` 경로 통일 |
| M4 | `app.go:70,109-112` | `tea.Quit` 시 `fifoCancel` 미호출 → `readFIFOFromPath` 고루틴 누수 | `Update`에서 `tea.QuitMsg` 가로채 `fifoCancel()` |
| M5 | `app.go:168` | `ListenForEvents` 재구독이 `context.Background()` 사용 → 영구 고루틴 누수 | `fifoCtx`를 `App`에 저장하고 전달 |

### Minor (9건)
| ID | 위치 | 문제 |
|---|---|---|
| m1 | `cmd/hook-handler/main.go:30` | `io.ReadAll(os.Stdin)` 무제한, OOM 위험 → `io.LimitReader(stdin, 1<<20)` |
| m2 | `fifo.go:193-194` | 4KB 넘는 FIFO write는 PIPE_BUF 초과, 동시 hook 시 byte interleave |
| m3 | `fifo.go:56` | `Mkfifo` 시 umask 의존 → 컨테이너 0666 위험, `Umask(0077)` 명시 |
| m4 | `jsonl.go:170-171` 외 | 1MB scanner ErrTooLong 침묵, base64 큰 응답 세션 preview 불완전 |
| m5 | `jsonl.go:566-644` | `ParseDAG`가 메인 JSONL 2회 read → race window |
| m6 | `recommender.go:96` | `DiscoverAllSessions`가 `context.Background()`로 호출, 탭 전환으로 취소 불가 |
| m7 | `github.go:160-166,243-247` | GitHub rate-limit 침묵 (log discard), 사용자는 빈 결과만 봄 |
| m8 | `github.go:283-295` | `fetchRepoStars`가 결과당 `gh api` 직렬 호출, 30개면 9-15초 추가 지연 |
| m9 | `recommender.go:189-208` | 캐시 무한 성장 (eviction 없음), write 에러 침묵 |

---

## 1.4 Related Projects & Research Landscape (Agent 4: Researcher)

> ⚠️ **신뢰성 경고:** 일부 인용(arXiv ID `2604.25602`, `2602.21227` 형식과 미래 시점 사건)은 형식상 무효이거나 미검증. 카테고리별 큰 흐름은 신뢰 가능하지만 구체적 paper/날짜는 직접 확인 후 인용 권장.

### 직접 경쟁/사촌
| Tool | 특징 | 이 프로젝트와 차이 |
|---|---|---|
| ccusage (npm) | 로컬 JSONL 사용량 분석 | 사후 분석 vs 실시간 orchestration |
| codeburn | TUI 비용 대시보드 | 사후적, DAG 없음 |
| Claudia/Opcode | Tauri+React 데스크탑 GUI | workspace/session 중심, hook 미통합 |
| claude-code-otel | Claude Code 지표 OTel export | **상호보완 가능** (OTLP 소비 또는 emit) |
| claude-code-router, claude-trace | routing/history 단편 | 단일 기능 vs 종합 control plane |

### LLM Observability 플랫폼 (2026)
LangSmith / Langfuse / Phoenix / Datadog LLM / Honeycomb LLM이 주류. **공통 한계:** 코드 인스트루멘테이션 의존, CLI 도구 호출 흐름에 약함.
**OpenTelemetry GenAI semconv:** client span 안정, agent-level span experimental. 이 프로젝트가 OTLP emit하면 Datadog/Honeycomb과 자동 interop.

### 오케스트레이션 프레임워크
LangGraph (LangSmith native), CrewAI AMP (실시간 observability), OpenAI Agents SDK (Swarm 후속, 자체 텔레메트리 약함), AutoGen, smolagents, Letta.
**공통 패턴:** structured event stream → SaaS SDK 소비.
**빠진 신호:** 대부분 linear trace만, **tool-call DAG**는 거의 없음 → 이 프로젝트의 진짜 niche.

### 2026-2027 변곡점
- **MCP 레지스트리 표준화** — `.well-known/mcp.json`. Skill marketplace를 **로컬 MCP discovery proxy**로 정렬
- **로컬 agent runtime 부상** — on-device 추론, API 의존 감소 → 이 프로젝트 아키텍처에 유리
- **규제 + audit trail 요구** — OWASP Agentic Skills Top 10 → native audit-trail tool 포지셔닝
- **스킬 보안** — registry poisoning 사례 증가 → 마켓플레이스 scanning 필수

---

# Part 2 — Cross-cutting Themes

네 리뷰가 독립적으로 도달한 공통 결론:

### Theme A — "외부 계약이 묵시적이라 모든 게 fragile"
- 리뷰 1: `Event.Payload`가 `json.RawMessage`, JSONL schema는 reverse-engineered
- 리뷰 3: hook stdin 무제한, FIFO write 비원자성, scanner ErrTooLong 침묵
- 리뷰 4: OTel GenAI semconv 미연결, MCP 메타데이터 미파싱

**공통 근본 원인:** Claude Code, GitHub API, 파일시스템, MCP — 4개 외부 계약과 만나는 지점에 **타입드 envelope + 버전 디스크리미네이터 + 명시적 fallback**이 없음.

### Theme B — "핵심 가치(DAG)가 데이터 모델까지 도달하고 UI 직전에 멈춤"
- 리뷰 2: `ParseDAG`/`DAGNode` 완성, `tab_dag.go` 미존재
- 리뷰 1: `DAGNode.Status string`이 sum type 아님, DAG 파싱이 god file에 매몰
- 리뷰 3: `ParseDAG`의 2-pass race window
- 리뷰 4: tool-call DAG는 경쟁 도구 중 거의 없는 differentiator

**기회:** UI 80줄 + Status sum type + 1-pass refactor + race fix = 1-2주에 시장 빈 슬롯 확보.

### Theme C — "Bounded context가 흐릿해서 변경 비용이 빠르게 증가 중"
- 리뷰 1: `internal/skill`이 services 잡탕, `Recommender`가 `internal/data` 직접 호출
- 리뷰 1: `HooksConfig`가 UI 패키지에 정의
- 리뷰 1: `jsonl.go` 777줄 god file
- 리뷰 2: `recommender.go`/`tab_agents` 잘라내기 권고

**현재 영향:** ~16K LOC에서 추가 기능 비용이 기하급수적으로 증가하기 직전. 도메인 재정렬 골든 타임.

### Theme D — "Local-first가 강점인 동시에 single-host로 인한 ceiling"
- 리뷰 2: multi-host aggregation 미지원, swarm 운영자에겐 한계
- 리뷰 4: OTel/Langfuse sink로 bridge 가능
- 리뷰 3: 그러나 그 sink조차도 계약(Theme A)이 정의되어 있어야 안전

**전략적 의미:** Sink 확장은 매력적이지만 Theme A를 먼저 해결하지 않으면 fragile한 계약을 외부에 노출 = 부채를 SDK로 굳히는 셈.

---

# Part 3 — Improvement Strategy Options

각 전략은 self-contained하고 4-6주 안에 의미 있는 결과를 낼 수 있는 단위.

### Strategy A — "Stabilize-First"
**가설:** 사용자가 한 번 터미널 깨짐을 경험하면 영구 churn. 가드레일 부채부터 청산.

**작업:**
- M1-M5 (signal, non-tty, ExecProcess, fifoCancel, fifoCtx) 모두 수정
- m1-m3 (stdin LimitReader, FIFO 4KB cap, umask) 수정
- `~/.gemini/cache/` → `~/.claude/cache/` 버그 픽스
- 깨진 awesome-skills URL 4건 제거 또는 수정
- `go mod tidy`, `flake.lock` 핀, `go.mod` 1.26.1 오타 수정

**완료 시점:** 1.5-2주
**산출물:** 신규 기능 0, 운영 신뢰성 step-function 상승

### Strategy B — "Ship the DAG"
**가설:** 시장에 비어 있는 가장 큰 슬롯 = 실시간 tool-call DAG. 데이터는 이미 있음.

**작업:**
- `tab_dag.go` 신설 (ASCII graph 렌더링, 80-150 LOC 예상)
- `Ctrl+D` 키바인딩 활성화 (README 표는 이미 있음)
- `DAGNode.Status` → `type NodeStatus string` + 상수
- `ParseDAG` 2-pass → 1-pass (m5)
- README screenshot + "Swarm Orchestration Debugging" 섹션

**완료 시점:** 2-3주
**산출물:** product hypothesis 검증, demo-able 기능

### Strategy C — "Contract-First Pivot"
**가설:** Theme A가 모든 fragility의 근본. 외부 계약 4개(Hook, JSONL, MCP, OTel)에 typed envelope 도입.

**작업:**
- `internal/event/hook` 신설: `PreToolUsePayload`, `PostToolUsePayload` + `SchemaVersion`
- `internal/data/jsonl.go` god file 분할: `header.go`, `conversation.go`, `dag.go`, `file_changes.go` + 공용 `scanner.go` (DRY)
- raw struct → typed (`SessionID`, `MessageID` newtype) + `ParseWarning` 반환 타입
- `HooksConfig` → `internal/data/config/` 이동
- `App.searchFocused` 등 3개 bool → `type AppMode int` sum type

**완료 시점:** 3-4주
**산출물:** 향후 신규 기능 개발 속도 영구 상승, 외부 스키마 변경 자가-진단 능력

### Strategy D — "Telemetry Sink Out"
**가설:** Local-first single-host 천장 돌파. OTLP emit으로 Datadog/Honeycomb 사용자 즉시 흡수.

**작업:**
- `internal/sink/` 인터페이스 + `otlp.go`, `langfuse.go`, `stdout.go` 구현
- FIFO event → sink fan-out 파이프라인
- OTel GenAI semantic conventions 매핑
- `--sink=otlp://...` CLI 옵션

**완료 시점:** 4-6주
**산출물:** Cloud bridge, B2B 채널 개방
**전제:** Strategy C 선행 (fragile 계약을 SDK로 굳히는 위험 회피)

### Strategy E — "MCP-Aware Skill Marketplace"
**가설:** MCP 레지스트리 표준화가 2026-2027 변곡점. 먼저 들어가면 trusted curator 자리.

**작업:**
- `internal/skill`을 bounded context로 재정렬 (model/infra/app 분리)
- MCP `.well-known/mcp.json` 파싱
- 보안 스캔 통합 (Snyk API 또는 정적 패턴)
- `recommender.go` 깨진 URL 4건 픽스, `~/.gemini/cache/` 픽스
- 또는 `recommender.go` **완전 제거** (Phase B 권고와 일치)

**완료 시점:** 5-7주
**산출물:** 마켓플레이스 신뢰도, MCP 생태계 위치
**리스크:** MCP 레지스트리 표준이 아직 moving target → 잘못 베팅 시 재작업

### Strategy F — "Multi-Track Sprint"
**가설:** 인원이 충분하면 4개 트랙 병렬 가능. 모든 P1을 동시에.

**작업:** A의 Major 5건 + B의 DAG + C의 envelope + D의 OTel stub
**완료 시점:** 4주 (병렬 시)
**리스크:** 솔로/소규모 팀 가정에서는 비현실적. 코드 머지 충돌 + context switching 비용 폭발.

---

# Part 4 — Trade-off Matrix

| 차원 | A: Stabilize | B: Ship DAG | C: Contract-first | D: Sink-out | E: MCP/Skill | F: Multi-track |
|---|---|---|---|---|---|---|
| **Effort (eng-weeks)** | 1.5-2 | 2-3 | 3-4 | 4-6 | 5-7 | 4 (병렬) |
| **Time-to-user-value** | 즉시(신뢰성) | 2-3주(가시) | 3-4주(간접) | 4-6주(B2B) | 5-7주(생태계) | 4주 |
| **신규 사용자 유입** | 낮음 | **매우 높음** | 낮음 | 중간(B2B) | 중간 | 높음 |
| **기존 사용자 churn 방지** | **매우 높음** | 중간 | 낮음 | 낮음 | 낮음 | 높음 |
| **하방 위험 (실패 시)** | 매우 낮음 | 낮음 | 중간(refactor 회귀) | 중간(SDK 굳힘) | **높음(MCP 표준 변경)** | 매우 높음(혼돈) |
| **상방 잠재력 (성공 시)** | 낮음 | **높음** | **매우 높음(복리)** | 중간-높음 | 높음 | 높음 |
| **downstream 잠금 해제** | 없음 | E (skill DAG 시각화) | **B, D, E 모두** | 새 B2B 채널 | B (마켓플레이스 표시) | 모두 |
| **downstream 봉쇄** | 없음 | 없음 | 없음 | 일부 sink 형태 잠금 | recommender 형태 잠금 | 없음 |
| **외부 의존(Anthropic/MCP/OTel)** | 낮음 | 낮음 | 중간(JSONL 변경 시) | **높음(OTel semconv)** | **매우 높음(MCP 미정)** | 높음 |
| **moat 강화** | 약함 | **강함(unique feature)** | 강함(품질) | 중간(commodity) | **강함(curator)** | 강함 |
| **테스트 부담** | 낮음 | 중간 | **높음(전 패키지)** | 중간(통합) | 중간 | 매우 높음 |

### 핵심 관찰
1. **C는 평가의 비대칭이 가장 크다.** Effort 중간, 상방 매우 높음, 하방 중간, B/D/E를 모두 잠금 해제. 단점은 user-facing value가 간접적이라 "보여줄 게 없음".
2. **B는 가장 데모성 있다.** README 스크린샷·HN 포스트·트위터 데모 가능. 하지만 sum type/race fix 같은 C의 일부가 필요.
3. **A는 보험.** 사용자 churn 방지가 신규 유입보다 보통 더 중요 (특히 dev tool은 한 번 실망 = 영구 이탈).
4. **D는 시기상조.** Theme A 미해결 상태에서 sink 노출은 fragile 계약을 SDK로 굳히는 anti-pattern.
5. **E는 베팅 색깔이 가장 강하다.** MCP 표준이 굳으면 큰 보상, 안 굳으면 sunk cost.
6. **F는 솔로/소규모 팀에선 비현실적.** 머지 충돌 + 컨텍스트 스위칭 비용이 병렬 이득을 잠식.

---

# Part 5 — Recommendation

### 권고: "C + A의 P0 4건 → B"  (Hybrid, 5-6주 시리즈)

**Sprint 1 (Week 1):** A의 P0 가드레일 4건 (M1-M4) — **1일 작업 × 4건**
- `signal.Notify(SIGTERM/SIGHUP)` → `p.Quit()` + `p.ReleaseTerminal()`
- `term.IsTerminal(stdout.Fd())` 사전 체크
- `tea.QuitMsg`에서 `fifoCancel()` 호출
- `ListenForEvents` 재구독에 `fifoCtx` 전달
- 추가: `~/.gemini/cache/` → `~/.claude/cache/`, 깨진 awesome-skills URL 4건 제거

**근거:** 비용 1주, 운영 신뢰성 step-function. 이걸 안 하고 B를 출시하면 DAG 데모 중 터미널 깨져 first impression 망함.

**Sprint 2-3 (Week 2-4):** C의 핵심 — Hook envelope + JSONL 분할 + `AppMode` sum type
- `internal/event/hook/` 신설, typed payload + `SchemaVersion`
- `jsonl.go` 4파일 분할 + `newJSONLScanner` helper
- raw struct 위에 `SessionID`/`MessageID` newtype
- `HooksConfig` → `internal/data/config/`
- 3개 bool → `AppMode` sum type

**근거:** 모든 다운스트림(B/D/E)을 잠금 해제. user-facing은 아니지만 4주 내 ROI 시작.

**Sprint 4-5 (Week 5-6):** B의 DAG 출시
- `tab_dag.go` (ASCII graph)
- `NodeStatus` sum type
- `ParseDAG` 1-pass 통합
- README screenshot + "Swarm Orchestration Debugging" 섹션

**근거:** 시장 빈 슬롯 점유. C 완료 후라 race/sum-type 부채 없이 깨끗하게.

**Sprint 6+ (Week 7+):** D와 E는 시장 반응 보고 결정
- DAG가 호응 = B 확장 (cross-session breadcrumb, cost-per-agent)
- B2B 인바운드 발생 = D (OTLP sink)
- MCP 표준 stabilize = E (skill marketplace MCP-aware)

### 잘라낼 것 (지금 결정)
- `internal/skill/recommender.go` — Agent 2가 명시적 제거 권고. 깨진 URL 4건이 cosmetic 픽스가 아닌 "왜 존재하는가?" 의문으로 확산. C 단계에서 함께 제거 검토.
- `tab_agents.go` — 활성 사용 흔적 없고 Hooks 탭에 흡수 가능. C의 패키지 정리 때 묶어서.

### 의도적으로 **하지 않는** 것
- **신규 sink/마켓플레이스/replay 기능** — C가 끝나기 전엔 부채를 SDK로 굳히거나 unstable 표준에 베팅하는 anti-pattern
- **Multi-track 병렬 작업** — 솔로/소규모 팀 가정 시 ROI 마이너스
- **OTel 통합** — semconv agent-level span이 still experimental, 6개월 뒤 다시 평가

---

# Appendix A — Process Notes

- **4 agents launched in parallel** via background `Agent` calls. Total wall time ~3분, sequential 추정치는 ~12분.
- **Agent 4 (researcher)의 일부 학술 인용은 미검증** — arXiv ID `2604.25602`, `2602.21227` 형식 무효, "2026 March Mintflare" 등 미래 사건. 본 로그에서는 카테고리별 큰 흐름만 채택하고 paper 인용은 보류.
- **Severity 합계:** Critical 3건 (Agent 1) + Major 10건 (1·3 합산) + Minor 14건. Critical 3건이 모두 *외부 계약 부재*라는 점이 Theme A의 강력한 근거.
- **본 문서의 한계:** 4개 에이전트 모두 read-only로 동작. 실제 동작 테스트(특히 가드레일 시나리오: `| tee`, SIGHUP, 동시 hook write)는 수동 검증 필요.
