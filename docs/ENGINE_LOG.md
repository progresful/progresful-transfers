# Engine log

What each engine version does, how it was judged, and which ideas were measured and rejected.
The point of this file is to stop the loop of "improve, then quietly undo": **a change becomes
the default only if it beats the champion on the fixed benchmark**, and every result here can
be traced to the exact source that produced it.

## How a version is judged

- **Build:** `scripts/build.sh` stamps `bin/hc` with the hash of its source and saves a snapshot
  to `runs/engines/<version>-<hash>.tar.gz`. Every run records engine version, source hash,
  the version note and the full params (`runs/<run>/meta.json`, `runs/index.jsonl`).
- **Benchmark:** `hc bench -level 8` plays the level's default params on eight fixed seeds
  (`spread-1, n2, n4, n5, b1, b2, b3, b4`). `hc versions` compares versions on those same weeks.
- **Champion rule:** highest mean margin among versions that are **ranked (≥ 90 % on time) on
  every week**. This mirrors the ranking rounds: five weeks, each must clear the minimum.
- **Noise floor:** the world reacts chaotically to small changes. On the eight weeks, settings
  that differ by less than about **€300–400** in mean margin are indistinguishable (risk cap
  0.42 → −75, 0.45 → −118, 0.5 → +341: not monotonic). Do not adopt on smaller differences.
- **One week is not evidence.** Seed `b1` is a rich world: every version earns €17.6–19.8k
  there. Thursday risk ×0.6 looked like +€770 on `b1` and was +€6 over the eight weeks.
- Sweeps (`hc sweep`) explore settings and are *meant* to include bad ones; only `bench` runs
  judge a version. `hc best` ranks every setting ever played by its average over the same seeds.

To rebuild an old version:
`mkdir /tmp/old && tar -xzf runs/engines/e2-62bdbc03836b.tar.gz -C /tmp/old && (cd /tmp/old && go build -o hc ./cmd/hc)`

## Versions (level 8, eight-week bench)

| Version | Mean margin | On time | Worst week | Ranked | Verdict |
|---|---:|---:|---:|---:|---|
| e1 | €12,304 | 94.2 % | 88.4 % | 7/8 | Rejected: one unranked week, and 1–2 undelivered cars in three weeks. |
| **e2** | **€11,868** | **96.0 %** | **92.2 %** | **8/8** | **Champion.** e1 + on-time protection (below). |
| e3 | €11,557 | 96.8 % | 93.8 % | 8/8 | Not adopted: refitted risk model is safer but −€311. Kept selectable (`riskModel=3`, `riskMax≈0.2`). |
| e4 | €11,868 | 96.0 % | 92.2 % | 8/8 | Same decisions as e2 (to the cent); adds experiment switches. |
| e5 | €11,570 | 94.8 % | 92.0 % | 8/8 | Rejected: quoting trade-ins, 47 % of them late, −€298. |
| e6 | €11,868 | 96.0 % | 92.2 % | 8/8 | Same decisions as e2; trade-ins declined on purpose. |
| e7 | €11,715 | 95.7 % | 91.6 % | 8/8 | Not adopted: lane price effects, −€153 (noise). Kept as a switch (`laneWeight`). |
| e8 | €11,868 | 96.0 % | 92.2 % | 8/8 | Same decisions as e2/e6. **Current code.** |

The eight bench weeks are on the hard side. The champion's defaults on 16 further worlds
(`c1`–`c16`) average **€14,403** (95.7 % on time, worst 92.7 %, 16/16 ranked); over all
24 distinct worlds: **mean €13,558, min €7,338, max €20,891, 24/24 ranked**. Level 8
practice rankings at the time (round "Saturday 00:00", other worlds): first €16.0k per
week, house-strong €14.4k, tenth €11.3k.

Lower levels with the same engine (bench, eight weeks, all ranked, untuned defaults):
level 7 €6,668; level 6 €3,819; level 5 €3,591; level 4 €3,874.

## What the champion does (and what each part was worth when it went in)

Measured on the seeds available at the time (2–4 seeds before the bench existed), so the
numbers show direction and rough size, not exact additive effects.

| Idea | Why | Measured effect |
|---|---|---|
| Buffered delivery window (arrive 15 % of the drive before the deadline) | Real drives run up to 20 % over the map minutes; drivers plan to arrive exactly at the deadline. | Level 1: 89.5 % → 95.2 % on time together with the next row. |
| Repost a gig that drivers can no longer book inside its windows | A gig posted while on-time was possible stays in the app but unbookable after its last on-time pickup. | (same run) €5.8k → €8.2k at level 1. |
| Scarcity cost: price on 1.3× the typical fee | In the shortage, winning every car you can sinks the on-time share. | Level 8: 57 % on time at 733 orders → ranked at ~300 orders. |
| Lateness-risk model + cap (driver supply, slack, pickup today, backlog nearby) | "Never booked before the last pickup" was 80 % of late orders. | €6.8k (blunt supply cutoff) → €8.2k, then €9.6k with the backlog term. |
| Note reader (99.8 % of noted cars read exactly on held-out weeks) | The world acts on notes; the order's `deadline`/`lastPickupAt` ignore them. | Failed visits 14/week → 0; one seed 88 % → 95.5 % on time. |
| Adaptive market level per world + per dealer, with dealer trust | Competitor level differs by world (1.26–1.54× list); trust adds up to 25 %. | Top dealer accepted 1.69× list at trust 20–25 vs 1.26× at 0. |
| Opening fee 0.8× typical, rising 5 % / 15 min to 1.15× | Cheapest drivers book in minutes; nobody is gained by opening lower. | +€778/week vs opening at 1.0× (4/4 seeds). |
| Decline premium cars | 18.6 % late, a third less margin per order (61 experienced drivers of 832). | Part of e1. |
| Day windows for notes that shorten site hours | Drivers plan by stated hours; one window per day keeps them inside the real ones. | +€250/week, no failed visits. |
| Forward-looking on-time guard | Late orders show in the account hours or days after they were won; a guard on refunds reacts too late (Thu/Fri). | e1 → e2: the unranked week 88.4 % → 92.2 %. |
| Thursday ×0.8 / Friday ×0.6 on the risk cap | Orders won Thu/Fri are late 8.5 % / 11.6 % vs ~4 % early in the week. | Part of e2. |
| Fees follow a changed destination; fee capped by the order's worth | A dealer can send a won car much farther; the old fee found no driver → undelivered (−2× price). | Undelivered cars 0.4/week → 0. |
| Full request tape + offline replay, engine versions, bench | Reproducibility; no silent regressions. | — |

## Measured and rejected (do not retry without a new reason)

| Idea | Result on the bench seeds |
|---|---|
| Direct offers at the app fee | 20 % read, 1–3 % of those accepted; thousands of actions for a handful of bookings. Off. |
| Higher fees to buy capacity (opening 1.3×, hold 1.7×, top 3×) | Margin fell sharply, on-time barely moved: capacity is not fee-elastic. |
| Flat "posted price" fee plans (0.9–1.0×) | Same margin, lower on-time; some weeks unranked. |
| Same-day price premium (1.08–1.2×) | −€550 to −€900. |
| Trust "investment" (cheaper early bids to build trust) | −€570 at 0.5, far worse at 1.5 (too much volume early). |
| Trust-aware risk cap / top-fee boost / won-share cap per dealer | Within noise or negative. |
| Volume target per driver (price follows the day's wins) | More orders, lower on-time, no margin. |
| Loosening when the on-time outlook is high | Within noise or negative: the marginal orders in easy weeks earn nothing. |
| Fast same-day market tracking at rate 0.05 | −€1,900 (unstable). Kept at 0.015 (+€120, within noise). |
| Friday cost premium (1.15–1.3×) | −€190 to −€320. |
| Cheaper cost for long hauls / dearer for short hops | costLong 0.92: −€588; 0.85: −€1,603; costShort either way: within noise. |
| Risk cap 0.42–0.5 on the champion's model | −€118 … +€341: noise. |
| Guard floor 0.94 | −€569 for +1.4 points on the worst week. |
| Quoting trade-ins | 47 % late, fees 1.43× typical: −€298 (e5). |
| Refitted risk model with weekday/hour/option (e3) | Better ranking of risk (AUC 0.74 vs 0.69), +0.8 points on time, −€311 margin. |
| Lane price effects (e7): pickup/delivery place shifts of the price level, fitted on 567k quotes | Real and stable (better win prediction on 24 of 24 held-out worlds), but −€153 at full weight, +€71 at half: noise. Most bids are set by the cost floor, which already follows lane difficulty through the typical fee. |
| Wider demand curve (market scale 0.12 instead of 0.07) | −€1,050. |
| Guessing the hidden ranking/final seeds | Not done: the seeds are secret on purpose (API.md), and seeds are arbitrary strings. |

## Known limits

- Margin sits on a plateau around €11.9k/week on the bench; knob tuning is exhausted at the
  noise floor. Further gains need something structural (multi-car chains, smarter offers).
- Where the margin is: orders over 300 km are a quarter of the orders and almost half of the
  margin (€70 per order against €18 for hops under 60 km).
- Premium cars and trade-ins are declined (about 7 % of requests).
- The risk model was fitted on our own practice weeks; a final week unlike them is covered
  only by the on-time guard.
