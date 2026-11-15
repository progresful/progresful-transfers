# Engine log

What each engine version does, how it was judged, and which ideas were measured and rejected.
The point of this file is to stop the loop of "improve, then quietly undo": **a change becomes
the default only if it beats the champion on the fixed benchmark**, and every result here can
be traced to the exact source that produced it.

Reading order: the Versions table is the index. The sections up to "Known limits" cover e2
to e10; those from "Saturday morning" to "The final" run forward in time; the sections
after "The final" are older (Saturday's test finals and ranking rounds, e16 and e17, e14,
e11 to e13, the lower levels).

## How a version is judged

- **Build:** `scripts/build.sh` stamps `bin/hc` with the hash of its source and saves a snapshot
  to `runs/engines/<version>-<hash>.tar.gz`. Every run records engine version, source hash,
  the version note and the full params (`runs/<run>/meta.json`, `runs/index.jsonl`).
- **Benchmark:** `hc bench -level 8` plays the level's default params on eight fixed seeds
  (`spread-1, n2, n4, n5, b1, b2, b3, b4`). `hc versions` compares versions on those same weeks.
- **Champion rule:** highest mean margin among versions that are **ranked (≥ 90 % on time) on
  every week**. This mirrors the ranking rounds: five weeks, each must clear the minimum.
- **Since e14, level 8 is judged on 24 worlds** (the eight bench seeds and `t1`–`t16`):
  the eight alone are too few and too easy on the on-time share (see "e14").
- **From e18 a change is judged paired per world, with the standard error of the
  difference**; from e21 on 48 worlds, from e23 on 96 (48 of them held out). The €300–400
  floor below belongs to the eight weeks: on the larger sets smaller gains were adopted
  when clear of their own s.e. (e22: +€86, s.e. 30), and e25 was adopted for its on-time
  share, not for its margin (+€53, s.e. 75).
- **Noise floor:** the world reacts chaotically to small changes. On the eight weeks, settings
  that differ by less than about **€300–400** in mean margin are indistinguishable (risk cap
  0.42 → −75, 0.45 → −118, 0.5 → +341: not monotonic). Do not adopt on smaller differences.
- **One week is not evidence.** Seed `b1` is a rich world: every version earns €17.6–19.8k
  there. Thursday risk ×0.6 looked like +€770 on `b1` and was +€6 over the eight weeks.
- Sweeps (`hc sweep`) explore settings and are *meant* to include bad ones; only `bench` runs
  judge a version. `hc best` ranks every setting ever played by its average over the same seeds.

From e9 on, every engine version is one commit with a tag (`git checkout engine-e14`); the
engine commits carry code only, this file is kept apart. Older versions exist as source
snapshots:
`mkdir /tmp/old && tar -xzf runs/engines/e2-62bdbc03836b.tar.gz -C /tmp/old && (cd /tmp/old && go build -o hc ./cmd/hc)`

## Versions (level 8, eight-week bench)

| Version | Mean margin | On time | Worst week | Ranked | Verdict |
|---|---:|---:|---:|---:|---|
| e1 | €12,304 | 94.2 % | 88.4 % | 7/8 | Rejected: one unranked week, and 1–2 undelivered cars in three weeks. |
| **e2** | **€11,868** | **96.0 %** | **92.2 %** | **8/8** | **Champion until e9.** e1 + on-time protection (see "What e2 did"). |
| e3 | €11,557 | 96.8 % | 93.8 % | 8/8 | Not adopted: refitted risk model is safer but −€311. Kept selectable (`riskModel=3`, `riskMax≈0.2`). |
| e4 | €11,868 | 96.0 % | 92.2 % | 8/8 | Same decisions as e2 (to the cent); adds experiment switches. |
| e5 | €11,570 | 94.8 % | 92.0 % | 8/8 | Rejected: quoting trade-ins, 47 % of them late, −€298. |
| e6 | €11,868 | 96.0 % | 92.2 % | 8/8 | Same decisions as e2; trade-ins declined on purpose. |
| e7 | €11,715 | 95.7 % | 91.6 % | 8/8 | Not adopted: lane price effects, −€153 (noise). Kept as a switch (`laneWeight`). |
| e8 | €11,868 | 96.0 % | 92.2 % | 8/8 | Same decisions as e2/e6. What it showed level by level: see "Where engine e8 stood". |
| e9 | €12,587 | 96.5 % | 92.8 % | 8/8 | +€718 (7 of 8 weeks): chains. Commit `0605e8f`, tag `engine-e9`. |
| e10 | €12,900 | 96.5 % | 93.8 % | 8/8 | +€313 over e9: chains over a hop of up to 70 minutes. Commit `c315f92`, tag `engine-e10`. |
| e11 | €12,952 | 96.7 % | 93.8 % | 8/8 | Level 8 decides as e10 except that a car whose real site hours fit no single day is declined (see "e11: the car that was never delivered"); levels 1–7 are retuned (see "Lower levels"). Commit `30942ce`, tag `engine-e11`. |
| e12 | €12,868 | 96.8 % | 92.8 % | 8/8 | Rejected, never committed: a delivery window on late gigs made them leave the app twice as often (−€84, five of eight weeks worse, week `b1` −€1,497). |
| e13 | €12,920 | 96.6 % | 94.2 % | 8/8 | e11 with pickup windows that end 20 minutes before a site's real closing: −€32 at level 8 (noise), +€55 at level 7, where e11's two failed visits are gone. Commit `44c9b68`, tag `engine-e13`. |
| e14 | €12,951 | 97.2 % | 93.5 % | 8/8 | Champion until e15, chosen on 24 worlds (see "e14"): level 8 is priced by lane and its cost rises with the lateness risk. A tie with e11–e13 on these eight weeks, +€317 (s.e. 252, inside the noise) and clearly safer on the 24. Commit `4827952`, tag `engine-e14`. Played the ranking round "Saturday 00:00". |
| e15 | €14,712 | 97.5 % | 95.4 % | 8/8 | The belief model of the market and the lane risk model at levels 7 and 8 (see "Saturday morning"). +€1,761 on the eight weeks with a third more orders (393 against 304) and a better worst week; on 24 worlds €16,127 against €14,006. Commit `d6868ff`, tag `engine-e15`. Played the ranking round "Saturday 09:00". |
| e16 | €14,712 | 97.5 % | 95.4 % | 8/8 | Decides as e15 at level 8 and at levels 1–6 (bench identical to the cent); level 7 has a cost of 1.2 instead of 1.1. Commit `cf34fc5`, tag `engine-e16`. Played the ranking round "Saturday 12:00" and the 14:00 test final. |
| e17 | €14,712 | 97.5 % | 95.4 % | 8/8 | Decides as e16 in a full week (bench identical to the cent). In a world joined after its first day: no learning from the past's requests, cost 1.1 (see "Test final, Saturday 14:00"). Commit `709b1ca`, tag `engine-e17`. |
| e18 | €15,066 | 97.1 % | 94.7 % | 8/8 | A cheaper fee clock at level 8: hold 1.05× rising to 1.25× before the deadline pressure, late cars stay at 1.0×. +€419 on the 24 worlds (s.e. €137), 97.0 % on time, worst week 94.7 % (see "e18: the weak weeks taken apart"). Commit `af9f5e0`, tag `engine-e18`. |
| e19 | — | — | — | — | e18 with the hold fee starting at 1.0× instead of 1.05×. On the 24 worlds €16,772: +€226 over e18 (s.e. €96, better in 17), +€645 over e17 (s.e. €167), 97.1 % on time, worst week 94.8 %. Not benched on the eight weeks alone. Commit `507d394`, tag `engine-e19`. |
| e20 | — | — | — | — | e19 with a chain discount (`chainCostGain` 0.25) and the trust rush for dealers with 8 % of the requests or more (`trustRush` 0.08). On the 24 worlds €17,066: +€294 over e19 (s.e. €130, better in 16), on time 97.0 %, worst week 93.4 %. Not benched on the eight weeks alone. Commit `74e9e67`, tag `engine-e20`. |
| e21 | — | — | — | — | e20 with a second chain attempt per order (`chainTries` 2). On 48 worlds (the 24 and 24 fresh ones) +€354 over e20 (s.e. €107, better in 35), on time −0.1 points, worst week 91.9 %. Commit `6518ade`, tag `engine-e21`. |
| e22 | — | — | — | — | e21 with a cheaper Friday (`fridayCost` 0.95, `riskFriday` 1.25). On 48 worlds +€86 over e21 (s.e. €30, better in 32), on time −0.15 points, worst week 92.5 %. Commit `d890d14`, tag `engine-e22`. |
| e23 | — | — | — | — | e22 with fee clocks that start over at level 8 (`dropFreshMin` 360, `chainRecoverMin` 90, `nightClock`, `chainNight`, `holdAwake`, `raceSafe`). +€515 over e22 on 96 worlds (s.e. 78, better in 74), on time +0.2 points. Commit `838f29f`, tag `engine-e23`, hash `6da13fe70519`. |
| e24 | — | — | — | — | e23 with the lane's fee in place of the noise of a request's typical fee (`refCap` 1.04, `refCapPrice`, `refCapRisk`). +€639 over e23 on 96 worlds, 48 held out (s.e. 75, better in 78), on time −0.1 points, no week under 90 %. Commit `04e5c96`, tag `engine-e24`, hash `1db09c594107`. |
| e25 | — | — | — | — | e24 with lower fee-clock caps on lanes where drivers book at once and more caution in worlds under 720 drivers (`easyCap` 0.87, `midCap` 0.94, `easyPrice` 0.95, `rescueFrom` 0.10, `thinRisk` 1.25). Against e24 on 96 worlds: the same money (+€53, s.e. 75), on time +0.22 points, in thin worlds +0.69 (s.e. 0.18), all 96 ranked. Commit `d101989`, tag `engine-e25`, hash `ef514da1ef79`. **The last engine: it played the final, third of 236 teams (see "The final").** |

The eight bench weeks are on the hard side. With the decisions of e2–e8 (the champion at
the time) the defaults on 16 further worlds (`c1`–`c16`) averaged **€14,403** (95.7 % on
time, worst 92.7 %, 16/16 ranked); over those 24 distinct worlds (not the 24 of the later
sections, which are the bench and `t1`–`t16`): **mean €13,558, min €7,338, max €20,891,
24/24 ranked**. Level 8 practice rankings at the time (other worlds): first €16.0k per
week, house-strong €14.4k, tenth €11.3k.

Lower levels before the e11 retune (bench, eight weeks, all ranked, untuned defaults):
level 7 €6,668; level 6 €3,819; level 5 €3,591; level 4 €3,874. From e11 on: €7,902 /
€6,256 / €6,064 / €6,556 (see "Lower levels"); level 7 is €8,808 with e15 and €8,461 from e16.

## e9: chains (the first gain above the noise since e2)

Two single-car orders go in one gig when the second pickup is within 25 road minutes of the
first delivery and both cars stay on time with at least 3 hours of room to start. The gig
opens at 0.9 of the two typical fees and is an attempt of 90 minutes: if no driver takes it,
it is withdrawn and both cars are posted alone (each order is tried in a chain once).

- Why: every earlier attempt to add volume cost on-time share, and in driver-poor weeks we
  move only 0.44–0.51 orders per driver. A chain lets
  one driver move two cars and saves them the trip home and the trip out.
- Measured on the bench: about 57 chain attempts a week, 36 booked (63 %), the rest split
  back; margin €11,868 → €12,587, on time 96.0 % → 96.5 %, worst week 92.2 % → 92.8 %.
- Tuned one setting at a time on the eight weeks (difference to e9):
  hop 15 min −€980, 45 min +€114 (6/8), 70 min +€313 (5/8), 100 min +€154 (6/8); fee share 0.8
  −€667, 1.0 −€360; attempt 45 min −€87, 180 min −€174; room 120 min −€8, 300 min −€386.
  So 0.9 / 90 min / 180 min stay, and the hop went to 70 minutes (e10: about 89 attempts a
  week, 56 booked).
- The server refuses a post for a car that is already in a live gig ("leg 0 is already in a
  live gig"), so a chain that races a booking cannot double-book a car.

## What e2 did, the base of every later engine (and what each part was worth when it went in)

Measured on the seeds available at the time (2–4 seeds before the bench existed), so the
numbers show direction and rough size, not exact additive effects. Changed since at level
8: the scarcity cost is 1.2× from e14 and rises with the lateness risk; the Thursday and
Friday factors on the risk cap are off from e15, where the lane risk model replaces them
(e22 lets Friday's cap be 1.25 times the week's); the fee still opens at 0.8× and from e19
holds at 1.0×, rising to 1.25× before the deadline pressure.

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
| Quote only the slowest delivery option a dealer accepts (`slowOptions=1`) | −€2,219 (0/8 weeks): orders 304 → 238, on-time +1 point. Dealers want the speed. |
| Truly flat fees, no ramp at all (1.0×, 1.15×, 1.3× typical) | €9,063 / €9,012 / €6,493 against €11,868; flat 1.0× unranked on 3 of 8 weeks. The clock books cheap drivers first and the ramp rescues the rest. |
| Cost split by when the car is ready (`costLater` 0.78–0.88, `costToday` 1.12) | −€491 to −€1,759. Bidding harder even for cars ready on a later day (fast, cheap, rarely late) adds orders but pulls on-time down to 93–94 %. |
| Per-gig fee model (`costByGig`, `openByGig`; fee by delivery-side drivers, readiness, weekday) | Cost by gig +€157 (6/8), opening by gig −€161, both +€43: noise. Kept as switches. |
| Opening fee 0.9× or slower steps at level 8 | −€334 to −€490; 0.8× with 5 % steps every 15 minutes stays. |
| Chains of three or four cars (`chainMaxCars`) | +€20 and −€85 against e10: an open chain that takes a third car is booked less often (51 and 48 chains booked a week instead of 56). |
| A second chain attempt per order (`chainTries=2`) | +€114 (4 of 8 weeks): noise on the eight weeks. Adopted later on 48 worlds (e21: +€354, s.e. 107). |
| Around e10: attempt 120 min, fee share 0.95, risk cap 0.45, room 120 min | +€72, −€29, −€82, −€433. |
| Cheaper price floor with chains (`costMult=1.2`) | 357 orders but 94.0 % on time and one week unranked: −€285. Volume still costs on-time. |
| A bar on the chain's booking chance (`chainMinChance` 0.35 / 0.45 / 0.55 / 0.65; the likeliest pairing wins instead of the shortest hop) | −€114 / +€41 / +€7 / +€83 against e10: noise. 72–80 attempts a week instead of 89 and as many booked (54–59), but no fewer late orders. Kept as a switch. |
| Chain-aware pricing (`chainCostGain`): a request that would make a chain with an open car is priced on a lower cost | 0.15: −€261. 0.3: +€110 with 30 more orders and 10 more booked chains, but 95.8 % on time and a worst week of 91.4 %. Noise, and less safe. Adopted later at 0.25 together with the trust rush, on 24 worlds (e20: +€294, s.e. 130). |
| The fee clock keeps running for cars split from a failed chain (`splitClock` 0.5 / 1) | −€32 / −€148: noise. |
| Opening fee 0.7× typical at level 8 (`feeStart`) | −€342 (296 orders): 0.8× stays. |
| Live driver activity as a supply signal (drivers seen in the app today, or who booked before, near the delivery) | Not built: on the champion's weeks it predicts a late or slow order no better than the roster count (AUC 0.61 against 0.61; 0.53 against 0.56). |
| This world's own driver count as a shift of the competitor's price level | Not built: after the fixed lane effects, the quote residuals show no trend with a place's driver count relative to its average (566k quotes, 25 worlds). |
| Lane price effects again, now with chains (`laneWeight=1`) | −€348 at level 8; with cost 1.2×: +€225 but 95.0 % on time and a worst week of 92.3 %; with cost by gig: −€28. At level 5: −€78. Noise or less safe; still off. |
| Cost that rises with the lateness risk (`riskSlope` 1 with cost 1.2×, 1.5 with 1.15×) | +€133 / −€180: noise. |
| A 5 % markup on every quote (`shareCap=0 shareMarkup=0.05`) | −€1,099: 252 orders instead of 306. The prices are not too low. |
| The lead rule at level 8 (`minLeadMin=120 minLeadDrive=0.5`) | −€13: the risk model already covers it. |
| A learned shift of the price level per delivery place (`destRate=0.02`) | −€182; with lane effects: +€120. Noise. |
| Higher scarcity cost (`costMult=1.4`) | −€1,078: 257 orders. With 1.2× at −€285, 1.3× stayed until e14, which prices on 1.2× with a cost that rises with the lateness risk (see "e14"). |
| Wider demand curve again (`marketScale=0.1`) | About −€1,500 on six of the eight weeks (255 orders); stopped. 0.07 stays. |
| Chain offers: a direct offer of an open gig to a driver whose current gig ends near its pickup (`chainOffers=1`) | 378 offers a week, 6 accepted, 165 declined: −€143. Every offer variant tried so far fails the same way. |
| Direct offers to drivers who just opened the gig (`offerViewers=1`) | ~4,000 offers a week; 26 % read, 2 % of those accept at the app fee (6.7 % at ≥ 1.0× typical). Margin within noise. A viewer who does not book needs a higher fee, not an offer. |

## What the champion's weeks show (e10, eight bench weeks, analysed 2026-11-14 morning)

- **Drivers see our gigs; most do not book them at any fee.** 80–90 % of a world's drivers
  open at least one of our gigs in the week and 300–470 do so on a day, but only 17–24 %
  ever book one (30–60 a day). A driver who lives within 30 km of the *delivery* books 31–42 %
  of the orders they look at, even at 0.8× the typical fee; everyone else books 2–6 %, and
  that share hardly moves between 0.8× and 2× (pickup-side drivers on a same-day car are
  the exception: 1.4 % → 6 % from 0.8× to 1.35×). This is why paying more buys little.
- **Who moves our cars** (per week): 96 cars in chains (fee 0.79× typical, booked in 36 min),
  64 by a driver for whom the gig ends at home (0.90×), 50 that start at the driver's home
  (1.05×), 80 with neither end at home (1.00×, 108 min), and only 16 as a driver's own
  follow-on after another of our cars.
- **Follow-on offers do not work even towards home.** Of the chain offers sent earlier
  (3,024 over eight weeks), 6.4 % of the read ones were accepted when the gig ended within
  30 minutes of the driver's home, 1.6 % when it ended farther away.
- **Why orders are late** (11.2 a week of 310): 53 % found their first driver after the last
  on-time pickup, 20 % were dropped or a no-show and rebooked too late, 14 % were booked in
  time but collected late, 12 % were collected in time and arrived late. 4 a week are more
  than 24 hours late (half the price back).
- **Chain attempts:** 63 % are booked, half of those within 26 minutes. Orders in a booked
  chain are late 1.2 % of the time, orders never in a chain 3.0 %, orders whose chain attempt
  failed 8.0 % (they are the hard ones, not harmed by the attempt: skipping the unlikely
  attempts does not lower the late count). What predicts a booking: a short way back from
  the chain's end to its start (≤ 60 min: 88–100 %, > 240 min: 39 %), a short total drive
  (≤ 90 min: 86 %, > 450 min: 21 %), early in the week (Monday 81 %, Friday 45 %), a first car
  that is not due today, and a partner that has not waited long. A logistic model on these
  (`chainChance`) has AUC 0.79 on held-out worlds and is well calibrated.
- **A quarter of the requests (about 230 a week) would make a chain with one of our open
  cars at the moment they arrive.**
- **Easy lanes are over-priced and hard lanes under-priced.** At the same quoted ratio the
  share of quotes won rises with the lane's fitted price effect (`lanes.go`): 16 % on the
  fifth of requests with the lowest effect (deliveries into the capital region, where
  drivers live) up to 57 % on the fifth with the highest (deliveries to remote towns).
  Level 5 shows the same (24 % → 64 %) and level 7 too (15 % → 58 %). By the roster: 233
  requests a week deliver where 250 or more drivers live within 30 km; we quote them at
  1.42× list (1.80× typical), win 36 %, and are late on 1.3 % of them. 159 a week deliver
  where fewer than five drivers live; we win 48 % of those and are late on 4.8 %.
- **Dealers are less price-sensitive than the engine assumes.** Pairs of runs that priced
  the same requests differently (same seeds) give a slope of the win chance of 8–10 per
  unit of log price (a width of 0.10–0.13), against the engine's 0.07. Quoting 4 % higher
  at level 5 (cost multiple 0.92 instead of 0.85) lost 16 % of the wins and still earned
  more.
- The price-war day shows: on the weakest day of a week we win 34 of 157 quotes (22 %) at
  1.34× list, on the other days 69 of 162 (42 %) at 1.51×.
- Bulk orders (batches of ten or more cars, five a week): we win 37 % of them and are late
  on 3.7 %, like everything else. Nothing special to fix.

## Known limits

- Up to e8 the margin sat on a plateau around €11.9k/week on the eight bench weeks, and knob
  tuning was exhausted at the noise floor. The gains since were structural: chains (e9, e10),
  the belief and lane risk models (e15), the fee clocks (e18, e19, e23) and the lane's fee
  (e24). e25 earns €17,298 on the same eight weeks.
- Where the margin is: orders over 300 km are a quarter of the orders and almost half of the
  margin (€70 per order against €18 for hops under 60 km).
- Premium cars and trade-ins are declined (about 7 % of requests).
- The risk model was fitted on our own practice weeks; a final week unlike them is covered
  only by the on-time guard.

## Saturday morning: why the leaders are ahead at levels 7 and 8, and what e15 changes

**The gap (public feed, round "Saturday 00:00", same five weeks for every team).** At
levels 1–6 e14 is second or third. At levels 7 and 8 it is 6th and 8th, and the teams
ahead all look alike: they win far more orders and are *more* on time.

| Level 8 | Margin / week | Orders / week | € / order | On time |
|---|---:|---:|---:|---:|
| freya (1st) | €17,185 | 427 | 40.3 | 97.3 % |
| skadi, razors, ppo, klmisbest (3rd–6th) | €16.0–16.1k | 424–458 | 35–38 | 96.0–96.8 % |
| e14 (8th) | €15,526 | 347 | 44.8 | 95.9 % |

Level 7 is the same picture (leaders 353–372 orders at 97.5–97.8 %, e14 285 at 96.8 %).
For e14 more volume always cost on-time ("volume still costs on-time", every cheaper cost
setting tried). So the leaders had something e14 did not: they knew *which* orders are safe.

**What was tried on the way and did not explain it** (all on the eight bench weeks unless
noted; e14 = €12,951, 97.2 %, 304 orders):

| Idea | Result |
|---|---|
| Cheaper cost for easy orders (`costByGig=1` with spread 0.25 / 0.15 / 0.05; `costMult` 1.0 or 0.9 with risk slope 2–3) | 294–387 orders, €12,648–13,371, on time down to 95.1 %. The extra 105 orders a week were fine themselves (3.8 % late, €35.7 net) but the orders both settings won were sold 4.7 % cheaper and were late 5.4 % instead of 2.9 %. |
| Narrower demand curve (`marketScale` 0.06 / 0.05 / 0.04) | +€183 / −€292 / +€67: noise. |
| Fee keeps rising after the discovery clock (`holdMax` 1.35 / 1.5 / 1.7) | −€108 / −€68 / −€518. On time +0.2 points at most. |
| Deadline pressure starts earlier for long gigs (`leadDrive` 1 / 2, `leadMin` 480) | −€66 / −€826 / −€380, on time +0.3 points. A long haul that has no driver has none at 1.5× either: stuck gigs are not fee-elastic. Both kept as switches. |
| Drivers' activity (completed gigs) as a weight on the driver counts | The fifth of the drivers with 29 or more completed gigs books 1.0 gigs a week, the others 0.3; but as risk features the weighted counts add nothing (AUC 0.747 against 0.746). |
| The typical fee over the list price as a price feature | Looked strong on requests whose winning price could be bracketed (+0.25 per log unit), but that sample is cut by our own quotes; on all real quote outcomes the effect is nil. Not used. |

**Three findings that do explain it.**

1. *Each request has a sharp winning price.* The bench seeds were played 130–260 times
   each, so every request has up to 200 quotes at different prices. Within one world the
   chance to win falls from 91–95 % just under a request's own threshold to 6 % just over
   it (±1 %). What looks like a wide demand curve is our ignorance of the threshold; its
   parts are the world level, the day (sd 0.067), the lane's two places and the dealer.
2. *Lateness belongs to the request.* Of the requests won at least 15 times across runs of
   one seed, two thirds are never late, and 5–10 % of them carry 65–70 % of all late
   orders (late in more than a quarter of the runs, whatever the settings). Those are
   long hauls won with little time (hauls over 500 km won 12–30 hours before their last
   on-time pickup: 21 % late; with 30 hours or more: 7 %; lanes of 30–150 km: 1 %),
   same-day cars on a Friday (17 %), after 14:00 (15 %) or when sixty orders are already
   won that day (27 %), and hops under 30 km (4 %, their fee is too small to bring a
   driver from another town). The old risk model had the drive length with the wrong sign
   and, by a bug in how its inputs were measured, counted every car as "ready today" and
   took weekday, hour and lead from the minute the car is ready instead of the answer.
3. *The running-rate market model learns the day too slowly and is over-confident.* Scored
   online on 132,126 real quote outcomes of 164 weeks (42 seeds): log-loss 0.654, and
   quotes it gave a 75 % chance won 61 %.

**The two new models.**

- **Belief model of the market** (`marketModel=2`, `beliefs.go`): a Gaussian belief about
  each part of the price level (world, day, dealer, pickup place, delivery place),
  updated with every answer by assumed-density filtering, so the first answers of a
  morning move the day factor a lot and the last hardly at all; place effects that hold
  in every world were fitted with the learner in the loop (one half of the seeds predicts
  the other as well as itself). Log-loss 0.558 (level 7: 0.521 against 0.632), calibrated.
- **Lane risk model** (`riskModel=4`, `risklane.go`, weights in `risklane_weights.go`):
  38 inputs measured from the dealer's answer: driver counts, lane length, typical fee,
  lead, weekday and hour, the option, same-day interactions, and the day's load (orders
  won today, orders still without a driver, orders won in the last two hours). Fitted on
  98,329 orders of 227 weeks in 25 worlds, old and new policy; leaving whole worlds out
  its AUC is 0.83 (supply model's inputs: 0.75) and half of the late orders are in its
  riskiest tenth. It also holds at level 7 (AUC 0.84). Its output is a calibrated chance,
  so the Thursday and Friday factors on the cap are off with it and `riskMax` is lower.

**How e15 was chosen** (level 8; bench = the eight weeks, fresh = `t1`–`t16`; e14: bench
€12,951 / 97.2 % / worst 93.5 % / 304 orders, fresh about €14,534, 24 worlds €14,006 / 96.4 % / 92.7 %):

| Setting | Bench | Fresh 16 | 24 worlds |
|---|---|---|---|
| First lane model (no same-day or load terms, inputs still measured the old way), cap 0.15 / 0.2 / 0.25, running-rate market | €13,515 / 97.4 % / 372 orders; €13,799 / 96.5 %; €13,912 / 96.1 % (worst 90.9 %) | cap 0.2: €15,424 / 95.9 % | |
| … cap 0.2 with beliefs | €14,175 / 96.3 % / 371 | €16,213 / 96.2 % / worst 92.5 % / 409 | €15,534 |
| Beliefs alone (supply risk model) | €13,045 / 97.1 % / 293 (+€94: noise) | | |
| **Refitted lane model + beliefs, cap 0.12 (e15)** | **€14,712 / 97.5 % / worst 95.4 % / 393** | **€16,835 / 96.6 % / worst 94.1 % / 433** | **€16,127 / 96.9 % / worst 94.1 %** |
| … cap 0.16 | €14,787 / 96.4 % / 94.3 % | €16,844 / 96.2 % / 93.5 % | €16,158 |
| … cap 0.2 | €15,242 / 96.4 % / 93.7 % | €16,783 / 95.8 % / 93.2 % | €16,269 |
| … cap 0.25 | €14,785 / 95.9 % / 93.0 % | | |
| … cap 0.16 or 0.2 with cost 1.1 | €15,002 / 96.0 % / worst 91.2 % / 454; €15,119 / 95.6 % / 90.8 % / 455 | | |
| … cap 0.2 with risk slope 2 | €14,643 / 97.0 % / 374 | | |
| Lane model with a steeper risk price (slope 3, cap 0.15) | €12,439 / 323 orders: taxing risk prices out the profitable hauls; the cap selects better | | |

The three caps earn the same within noise; 0.12 is the safest, and the final is one world.
The belief model alone was noise on the bench, but with the lane model it is worth +€376
there and +€789 on the fresh worlds. A cost of 1.1 reaches the leaders' volume (455 orders)
without more margin and with a worst week near the minimum: the fees rise to 64 % of the
revenue (61 % at cost 1.2), the sign of too many cars for the drivers.

**Level 7** uses the same two models (the lane model was fitted at level 8 and holds at
level 7). Bench: cost 1.1 with cap 0.15 gave €8,808 / 96.9 % / 377 orders against e14's
€7,902 / 97.7 % / 268, and that became e15's default. **The ranking round then showed it
is too loose**: see below, and "e16" for the fix.

## Ranking round "Saturday 09:00": e15 (played 2026-11-14 11:35–11:45)

All 40 runs ranked. (Other weeks than the round before, so not comparable with e14's
€15,526 there: this round's level 8 weeks are poorer, the leader has €15,801.)

| Level | e15 | Place | Leader |
|---|---:|---|---|
| 8 | **€15,503** (97.7 % on time, 410 orders, worst week 96.6 %) | **3 of 59** | Juhus €15,801, DaVinci's Night Shift €15,515 |
| 7 | €11,213 (94.6 %, 460 orders, weeks 93.2 % and 90.0 %) | 7 of 16 | klmisbest €11,822 (393 orders, 96.6 %) |
| 6 | €7,763 | **1 of 16** | — |
| 5 | €5,706 | **1 of 4** | — |
| 4 | €7,048 | 2 of 5 | Northbound Dispatch €8,057 |
| 3 | €14,329 | 2 of 5 | Northbound Dispatch €15,964 |
| 2 | €10,371 | 2 of 5 | Northbound Dispatch €11,531 |
| 1 | €11,673 | 2 of 5 | Northbound Dispatch €12,024 |

- **Level 8:** eighth in the round before, third now, €298 a week behind the first, with
  the order count and on-time share of the leaders (410 orders, 97.7 %).
- **Level 7 is overloaded:** 408–522 orders a week where the leaders take 340–390, at €24
  an order against their €30–34. The fees are 66–74 % of the revenue, one week had 112
  dropped bookings and 50 late cars and ended at exactly 90.0 %. The eight bench weeks
  did not show it (377 orders there): level 7 needs its own validation on more worlds
  and a higher cost.

## Ranking round "Saturday 15:00", level 8 only: e17 (played 2026-11-14 15:25–15:31)

Five runs, all ranked: **€17,890 a week, 451 orders, 96.1 % on time: first of 25 ranked
teams at 15:32** (second akanegakirru €17,053, third klmisbest €16,988; this was early in the
round: source-exp later led it with €18,566, see "e18").

| Week | Margin | Orders | On time | Late cars | Bookings given back | Fees / revenue |
|---|---:|---:|---:|---:|---:|---:|
| 1 | €17,265 | 449 | 92.7 % | 32 | 90 | 62 % |
| 2 | €19,812 | 460 | 98.5 % | 7 | 67 | 57 % |
| 3 | €13,272 | 404 | 93.8 % | 24 | 73 | 66 % |
| 4 | €16,549 | 444 | 96.8 % | 14 | 53 | 59 % |
| 5 | €22,549 | 497 | 98.0 % | 10 | 53 | 56 % |

Weeks 1 and 3 are the ones to look at: on time 92.7 % and 93.8 % (the minimum is 90 %),
many bookings given back by drivers (90 and 73), 20 and 12 gigs that ran out of time, and
one cheap car left undelivered in each (penalties of €61 and €98). The leaders were late
in the same weeks (akanegakirru 95.7 % and 93.6 %, freya 95.5 % and 92.5 %), so these are
hard weeks, but e17 is two to three points under them in week 1.

## e18: the weak weeks taken apart, and a cheaper fee clock (2026-11-14, 15:40–16:50)

Asked for: the lessons of the 15:00 round and very small changes with a positive result.
Level 8 only; 24 full weeks per setting (e17 there: €16,127, 96.9 % on time, worst week
94.1 %, 417 orders). One caution learned on the way: the same world moves by €500–2,000
when any setting changes (its week takes another course), so a 24-world mean has a
standard error of about €140–200. Paired per world, not by the means alone.

**Why orders were late** (14 weeks of e17, 3,740 orders, 3.2 % late):

| Cause | Orders | Share |
|---|---:|---:|
| No booking before the last on-time pickup | 51 | 1.4 % |
| Booked in time, given back by the driver, booked again too late | 39 | 1.0 % |
| Booked in time, arrived up to 3 hours late | 20 | 0.5 % |
| Booked in time, more than 3 hours late | 9 | 0.2 % |
| Never delivered | 2 | 0.1 % |

Two thirds of the first two rows are hauls over 200 km. A late order still earns (about
€30 against €38): the refund is 10 % up to two hours, 25 % up to a day, 50 % up to two business days and
the whole price after that.
The two weak weeks of the round paid €1,978 and €1,288 in refunds.

**What the drivers do** (five ranking weeks and 40 practice weeks):

- **A driver looks at a gig once** (92 % of driver–gig pairs have one view). The fee shown
  at that moment is the one that counts.
- **A new gig is seen 26 times an hour in its first quarter of an hour, 13 after an hour,
  5 after four hours.** Posting an old gig again does not bring the looks back (6.9 an hour
  before, 6.8 after; a third of the new looks are drivers who had seen it).
- **Bookings per look do not rise with the fee**: 3–5 % at 0.8–0.9× the typical fee, 3–4 %
  at 1.0–1.1×, 2–3 % above. What decides is how far the gig takes a driver from home: 4–7 %
  per look when its ends are within 100 km of home, 1–2 % when 400 km of the round are unpaid.
- **Drivers give back 15 % of their bookings**: 16–21 % of those under 0.95× the typical
  fee, 12 % at 1.05–1.2×, 1–4 % above; 17 % of bookings made more than 12 hours before the
  last pickup, under 3 % of the later ones. Nine in ten of those orders still arrive on time.
- **Real drives**: the map's minutes on the median, 1.11× at the 90th percentile, 1.16× at
  the 97th; in forecast snow 1.24× on the median and up to 1.45×. An order picked up with
  less than 5 % of its drive to spare is late nine times in ten, with 25–40 % to spare 0.6 %.
- **Snow**: orders touching a place with forecast snow during the snow were late 7–12 %
  of the time against 3.6 %. They are 5 % of the orders.
- **The risk model is right on average and wrong by world**: in worlds with under 720
  drivers the weeks were late 1.3–1.4 times as often as modelled, with 720–880 drivers 0.7
  times. Both weak weeks of the round had under 700 drivers (688 and 659).
- **The leader of the round (source-exp, €18,566 a week against e17's €17,890) is ahead in
  exactly those two weeks**: €20,373 with 392 orders against our €17,265 with 449, and
  €14,115 with 365 against €13,272 with 404. In the other three weeks e17 is level or
  ahead. It moves 0.51–0.57 orders per driver in every week; e17 0.54–0.56 in the normal
  weeks and 0.61–0.65 in the thin ones.

**Settings tried** (24 worlds unless noted; margin and on-time against e17):

| Setting | Margin | On time | Worst week | Verdict |
|---|---:|---:|---:|---|
| Buffer 25 % of the drive (`bufferShare=0.25`) | −€56 | +0.2 | 94.9 % | Neutral; kept as it was. |
| The same with 45 % in forecast snow (`snowBuffer=0.45`) | −€127 | +0.1 | 93.7 % | No gain: the cars that arrive just late are 0.5 % of the orders. Switch kept. |
| Risk cap 0.10 (`riskMax`) | −€360 | +0.2 | 92.4 % | Rejected: the risky orders earn more per order than the safe ones. |
| Top fee under deadline pressure 1.3× instead of 2× (`feeMax`) | +€51 | −0.7 | 93.3 % | Rejected: the surge does bring cars in. |
| Top fee 2.5× | −€179 | −0.1 | 93.4 % | Rejected: 2× is enough. |
| Late cars' fee up to 1.2× instead of 1.6× (`lateMax`) | +€99 | 0.0 | 94.2 % | Good. |
| Late cars' fee stays at 1.0× | +€118 | 0.0 | 94.2 % | Good: in e18. |
| Hold fee 1.05× instead of 1.15× (`feeMid`) | +€302 | −0.7 | 92.8 % | More margin, more late cars. |
| Hold fee 1.0× | +€333 | −0.7 | 93.5 % | The same. |
| Hold 1.0×, late cars 1.2× | +€468 | −0.8 | 93.2 % | The same. |
| Hold 1.05× rising to 1.25× before the pressure (`holdMax`) | +€212 | −0.1 | 94.0 % | The rise wins the late cars back. |
| **Hold 1.05× rising to 1.25×, late cars 1.0×** | **+€419** | **+0.1** | **94.7 %** | **e18.** Paired: s.e. €137, better in 16 of 24 worlds; the six thin worlds +€229 and +0.2 points. |
| Six thin worlds: risk × 1.5 / × 2 under 720 drivers (`thinRisk`) | −€816 / −€1,088 | +1.4 | 95.2 % / 94.2 % | Buys on-time dearly. Switch kept. |
| Six thin worlds: cost 1.35 | −€1,195 | +1.7 | 95.5 % | Rejected. |
| Six thin worlds: cost follows wins per driver, target 0.57 (`targetPerDriver`, never under 1.2) | −€690 (gain 2), −€360 (gain 1) | +1.4 / +0.9 | 95.1 % | Rejected: fewer orders earn less here, unlike the leader's thin weeks. What it does differently there is still open. |

e18 on eight worlds joined on Thursday: €4,964, 97.8 % on time, 160 orders (e17: €4,970,
97.8 %, 161). Built defaults against the swept setting on one world: the same to the cent.
Commit `af9f5e0`, tag `engine-e18`. Levels 1–7 decide as e17.

## e19: the hold starts at the typical fee; what else was tried (2026-11-14, 16:50–17:31)

All on the 24 worlds, on top of e18's clock (hold rising to 1.25×, late cars at 1.0×):

| Setting | Margin | On time | Worst week | Verdict |
|---|---:|---:|---:|---|
| e18: hold from 1.05× | €16,546 | 97.0 % | 94.7 % | |
| **Hold from 1.0×** | **€16,772** | **97.1 %** | **94.8 %** | **e19.** Paired with e18: +€226, s.e. €96, better in 17 of 24. |
| Hold from 0.95× | €16,669 | 96.8 % | 93.3 % | Too low. |
| Hold from 1.05× rising to 1.35× | €16,432 | 97.1 % | 93.7 % | The higher end pays nothing. |
| Hold from 1.0×, opening fee 0.75× (`feeStart`) | €16,624 | 96.8 % | 93.7 % | 0.8× stays. |
| Hold from 1.0×, a step every 20 minutes (`stepMin`) | €16,600 | 96.8 % | 94.3 % | 15 minutes stays. |
| Hold from 1.0×, trust at stake × 4 (`trustGuard`) | €16,628 | 97.0 % | 94.5 % | No gain. |
| Hold from 1.0×, trust investment 0.5 on requests with a risk of 0.02 or less (`trustInvestRisk`, new) | €16,659 on 20 worlds | 96.8 % | 92.5 % | 439 orders instead of 414 and no more margin. Switch kept. |

e19 on eight worlds joined on Thursday: **€5,206, 97.9 % on time, 159 orders** (e18 €4,964,
e17 €4,970). Built defaults against the swept setting on one world: the same to the cent.

**The lateness model was not refitted.** Asked: it was fitted on older engines' weeks. On
216,208 single-car orders of 489 weeks played by e15–e18, the weights in the engine score
AUC 0.767 and log-loss 0.1289 and are calibrated in every band of risk (0.029 modelled,
0.031 late; 0.089, 0.089; 0.109, 0.120). A model fitted on those weeks and tested on
worlds it had not seen: AUC 0.766, log-loss 0.1291. The number of drivers in the world as
a feature makes unseen worlds worse (0.760, 0.1297): with 26 worlds a fact about the
whole world is learned by heart.

**Trust is mostly one dealer.** D01 sends about 300 of a week's 1,000 requests and gives a
third of our orders. Its trust is at the top (25) by Tuesday or Wednesday: its orders sell
at 1.32–1.52× the list price with no trust and 1.50–1.66× at the top. A late delivery
halves it; that happens to D01 2–11 times a week. Priced out over 29 weeks, the halvings
cost €160 a week in all (D01 €92), because most of them fall on Friday evening or later,
when no request is left to sell into. That is the ceiling for anything that protects trust.

**The game server restarted at 16:00 and at 17:07** (HTTP 502 for about a minute). Runs
in play stop with an error. A ranking week that was cut must be played to its end with
`hc rehearse -level N -world <id> -keep` (the live path joins the world; without `-keep`
the world is deleted when the command ends, and the server then plays a ranking week out
without us).

## After e19: leverage, the competitor's mornings, and its outage (2026-11-14, 17:35–)

**Trading on-time share for orders does not pay** (asked: we deliver 97 %, the minimum is
90 %; how much is there to gain by taking more?). On the 24 worlds, on top of e19 (€16,772,
97.1 % on time, worst week 94.8 %, 414 orders):

| Setting | Margin | On time | Worst week | Orders |
|---|---:|---:|---:|---:|
| Risk cap 0.18 instead of 0.12 | +€154 (s.e. €170, better in 14 of 24) | 96.2 % | 93.7 % | 419 |
| Cost 1.1 instead of 1.2 | +€122 | 96.3 % | 91.5 % | 468 |

Fifty-four more orders a week earn €122 together. A late car pays back a quarter of its
price (half after a day), halves its dealer's trust, and every extra order bids for the same
drivers, so the fees of all orders rise. The worst week comes within 1.5 points of the
minimum; a final played in a world with few drivers would be at risk of not being ranked.

**The competitor's morning moves by dealer are small.** The handbook: each morning it
lowers its prices for a dealer whose requests we mostly won and raises them where we won
few. Measured on 1,396 dealer-nights of 150 weeks (the price level a dealer accepted from
us, less the lane's fixed effects and the dealer's trust, against the other dealers of the
same morning): after a day with under 25 % of a dealer's requests won its level is 1.6 %
higher, at 25–75 % it does not move (±0.3 %), over 75 % it is 3 % lower (12 cases). The
engine wins 25–75 % of nearly every dealer's requests, so there is nothing to steer.

**The competitor's outage is real money.** The handbook: one working day a week its
systems are down in one region (a town and the places within 80 km) from 08:00 to 18:00,
and the dealers there pay a backup about 30 % more; busy regions go down more often.
Found in the logs of all 24 worlds: on one day the engine wins 72 % of its quotes in one
region against 34 % elsewhere, and in 13 of the weeks 84–99 %. It is nearly always the
capital region, with 60–100 quotes that day. The engine's prices there rise by 6 % during
the day (a win only says the price was not too high), where the backup would allow about
25 %. The outage watch (`outage.go`, switch `outage`) keeps, for every town, the odds that
its region is down today, moves them with every answer by how much likelier the answer is
on the backup's prices, and quotes the price that pays best over both cases.

It does what it was built for and earns nothing. In the weeks with a clear outage it
priced 72–102 quotes of that day on the backup from 09:40–11:20 on, at 1.65–1.88× the
list price against 1.41–1.58× elsewhere, and still won 61–72 % of them. But e19 wins
85–99 % there at about 6 % over its usual price, and the two come to the same money: the
win curve is wide against a 30 % step, and a third of the price is the driver's fee.
False alarms (a short run of wins in a region on a normal day) cost one to fifteen lost
quotes on one day in five, because every town of a busy area has its own overlapping
region that must be proven wrong by a lost quote of its own. On the 12 worlds that
finished (the server was restarted at 18:00): +€42 a week (s.e. €180), on time +0.3
points. The watch stays a switch, off. What an outage day brings is orders, not price:
60–90 wins in one region in a day, which the risk model's count of the day's wins
already holds back.

**Trust comes with the calendar, not with the dispatch.** The three biggest dealers send
half of all requests (the biggest alone 29 %), and their trust stands at 2.6 on Monday
evening, 12.8 on Tuesday, 17.1, 19.6 and 20.8 on Friday. Their orders sell at 1.35× the
list price on Monday and 1.47–1.57× from Tuesday on. Tried: post a big dealer's car to
arrive on the day it is ready, with the order's own deadline as the fallback when no
driver takes it in time (`trustRush`, `rush.go`). The rush itself works (59 % of the rush
gigs are booked within the hour at 1.01× the typical fee and all of those arrive that
day), but only 13 orders a week qualify: 57 % of these dealers' cars are ready the day
after the request and 91 % of those arrive on their ready day anyway; of the third that
is ready at once, half arrive the same day already. Monday's trust moves from 2.6 to 2.9.
On the 24 worlds: share 0.08 (three dealers) +€73 (s.e. €92), on time +0.2 points; share
0.2 (the biggest only) −€10 (s.e. €128). Switch kept, off.

**Premium cars stay declined.** 51 requests a week are refused because the car is premium
(only experienced drivers may move it). With `premium=1` the lane risk model lets nine a
week through (its premium term puts the rest over the cap); they are late 6.4 % of the
time and earn €32 each. On the 24 worlds: +€68 (s.e. €115), on time −0.45 points.

**Fees and lateness by the kind of car** (216,208 orders of e15–e19): a car ready on a
later day costs 0.94× the typical fee and is late 3.3 % of the time, a car ready on the
day of the request 0.98× and 3.2 %; hauls of 300 km and more cost 0.98× and are late
5.4 %, lanes under 60 km 0.92× and 3.3 %, the rest 0.94× and 2.6 %. The differences are
a twentieth of the fee, not the fifth that would make a cost per kind of car worth much.

**Old rejections tried again on e19** (they date from the eight-week bench and the risk
model before e15):

| Setting | Margin | On time | Worst week | Verdict |
|---|---:|---:|---:|---|
| Chain discount: a request that would share a driver with a car already held is priced on a cost lower by 0.25 × the chance that the chain is booked (`chainCostGain`) | +€166 (s.e. €137, better in 15) | −0.2 | 94.6 % | 24 more orders, 90 chains booked a week instead of 83. In e20. |
| A car ready on a later day priced on 0.93× the cost (`costLater`) | −€142 (s.e. €123) | −0.3 | 92.9 % | Rejected again. |
| Narrower dealer width of the price beliefs, 0.055 (`beliefScale`) | not judged here; later +€52 (s.e. €113) on 36 worlds, undecided and never adopted (see "Midnight screens on e21") | | | Only 8 of 24 worlds finished (the server could not be reached from 18:51 to 19:02). |

## e20: chain discount and trust rush together (2026-11-14, 19:23–19:44)

Asked for before the 20:00 test final: an e20 with the changes that had come out
positive. The two that each earned a little on e19 went in, the chain discount (0.25) and
the trust rush (0.08); premium cars (on time −0.45 points) and the outage watch (no gain,
false alarms) did not. Together on the 24 worlds, paired with e19: **+€294 a week (s.e.
€130, better in 16 of 24)**, 427 and 447 orders in the two halves of the run against 414,
on time −0.1 points, worst week 93.4 % against 94.8 %. The 24 worlds were played in two
parts (10, then the 14 that the first pass lost to connection failures). On eight worlds
joined on Thursday: €5,206 as e19, 166 orders against 159, 98.3 % on time against 97.9 %,
worst week 97.2 %. A replay of a swept week through the built defaults: the same
decisions. Commit `74e9e67`, tag `engine-e20`. Levels 1–7 decide as e19.

The worst week is the caution: one world fell from about 95 % to 93.4 %. e20 takes 5 %
more orders than e19, and the on-time share is what pays for them.

**On top of e20** (24 worlds, paired with e20):

| Setting | Margin | On time | Worst week | Verdict |
|---|---:|---:|---:|---|
| Direct offers for gigs open four hours (`offers=1 offerAfterMin=240 offerReach=0`) | −€58 (s.e. €111) | +0.1 | 94.4 % | 7,583 offers a week: 6,272 never read, 1,301 declined, 10 accepted. Rejected again; in a final it would also eat the action budget. |
| Chain discount 0.4 instead of 0.25 | −€159 (s.e. €132, better in 9) | −0.2 | 92.9 % | 0.25 stays. |

## e21: a second chain attempt (2026-11-14, 21:30–22:00)

The chain settings date from the eight-week bench (a second attempt per order was "+€114,
noise" there). On e20 and 48 worlds each, paired:

| Setting | Margin | On time | Worst week | Verdict |
|---|---:|---:|---:|---|
| **A second chain attempt per order (`chainTries` 2)** | **+€354 (s.e. €107, better in 35 of 48)** | −0.1 | 91.9 % against 92.3 % | **e21.** First 24 worlds +€305, fresh 24 +€402. |
| Three attempts | +€397 (s.e. €99); +€43 over two (s.e. €80) | −0.1 | 92.1 % | The same as two. |
| A hop of up to 100 minutes between the cars (`chainHopMin`) | −€315 on 24 (s.e. €118) | −0.1 | 93.5 % | Rejected. |
| The chain's fee 0.95 of the two typical fees (`chainFeeShare`) | −€112 (s.e. €111) | −0.1 | 90.1 % | Rejected. |
| Chains of three cars (`chainMaxCars` 3), on e21, 42 worlds | −€174 (s.e. €152) | 0.0 | 90.5 % | Rejected. |
| A chain attempt of 60 minutes instead of 90 (`chainTryMin`), on e21 | −€189 (s.e. €114) | 0.0 | 90.3 % | Rejected. |
| A chain attempt of 120 minutes, on e21 | −€76 (s.e. €103) | +0.1 | 92.1 % | The same as 90. |
| Only chains with a booking chance of 0.45 or more (`chainMinChance`), on e21 | −€30 (s.e. €118) | +0.1 | 91.4 % | The same. |
| Cars of a failed chain keep their fee clock (`splitClock` 1), on e21 | −€349 (s.e. €95) | +0.1 | 91.0 % | Rejected: opening them at 0.8× again is right. |
| Two hours of room to start a chain instead of three (`chainSlackMin` 120), on e21 | −€171 (s.e. €108) | 0.0 | 91.9 % | Rejected. |

What the second attempt does: 189 chains are tried a week instead of 134 and 111 booked
instead of 90; the orders won are the same (436–443 a week), the fees fall from 0.944 to
0.938 of the typical fee, drivers give back 55 bookings instead of 58, and as many legs
arrive late (14.7 against 14.4). The week it costs is `t13` (671 drivers): 93.4 % on time
with e20, 91.9 % with e21. On eight worlds joined on Thursday: €5,421 against €5,206, on
time 97.6 % against 98.3 %. A replay of a swept week through the built defaults: the same
decisions. Commit `6518ade`, tag `engine-e21`. Levels 1–7 decide as e20.

## The dealers, one by one (48 weeks of e20; asked: do some dealers send more, and should the price differ by dealer?)

Forty dealers in every world, and their numbers mean the same in every world: D01 is
always the biggest.

| Dealer | Requests a week (range over worlds) | Share | Orders won | Won of real quotes | Level (× list) | Won at | Late | Margin an order | Margin a week | Share of the margin |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| D01 | 309 (248–391) | 29.8 % | 145 | 73 % | 1.53 | 1.49 | 3.1 % | €44.0 | €6,378 | 37.4 % |
| D02 | 128 (87–168) | 12.4 % | 57 | 68 % | 1.47 | 1.46 | 3.6 % | €41.4 | €2,356 | 13.8 % |
| D03 | 86 (44–124) | 8.3 % | 38 | 70 % | 1.44 | 1.44 | 3.4 % | €39.7 | €1,509 | 8.8 % |
| D04 | 59 (29–127) | 5.7 % | 24 | 66 % | 1.40 | 1.40 | 3.0 % | €36.5 | €872 | 5.1 % |
| D05 | 44 (19–77) | 4.2 % | 19 | 66 % | 1.39 | 1.37 | 3.8 % | €35.1 | €656 | 3.8 % |
| D06–D12 | 184 | 17.8 % | 71 | 61 % | 1.35 | 1.36 | 3.2 % | €34.9 | €2,494 | 14.6 % |
| D13–D40 | 225 | 21.7 % | 84 | 59 % | 1.33 | 1.34 | 3.3 % | €33.6 | €2,807 | 16.4 % |

The three biggest send half of all requests and bring three fifths of the margin. The
level a dealer accepts rises with its size because trust does (25 on-time orders are
reached by Tuesday at D01 and never at a dealer with ten requests a week); the engine
already prices each dealer against its own level.

**Price sensitivity is the same for every dealer.** Win against the quote's distance from
the level, 31,700 real quotes: slope 11.5 at D01, 11.3 at D02, 10.9, 11.0, 11.5, then 11.1 for
the seven next in size and 11.2 for the smallest 28 (a width of 0.09); at the level every group wins 54–56 %,
3–6 % under it 66–70 %, 3–6 % over it 41–46 %. So there is no dealer to mark up more than
its level says. Within one world D01's slope runs from 5.5 to 17.7 (quartiles 10.0, 10.8,
12.9): a trait of the week that 200 quotes could learn by Wednesday, worth about a
percent of that dealer's margin at most.

**A coefficient tried out on Monday has nothing to learn from.** What a cost of 1.1
instead of 1.2 changes in a world's week (58 world comparisons on e19 and e20) does not
follow anything Monday shows: the driver count, orders won per driver, the share of gigs
booked within the hour, the fee paid over the typical fee, bookings given back, looks per
gig, the share of quotes won, the price level; every correlation with the margin change
is between −0.07 and +0.11. The same world moves by ±€1,000 when any setting changes, so
one day's result of a randomised coefficient is noise.

**Premium cars, once more, with their own cap.** `premium=1` and `premiumCap=0.45` on e21
(24 worlds): +€27 (s.e. €156). Of 51 premium requests a week 41 are priced and 12 won:
the risk makes their price dear, and a dealer pays no more for a premium car (list price
€82 against €81). Those orders are late 12.8 % of the time against 2.9 % for the rest, at
a fee of 1.06× the typical fee, and earn €32 against €39. By modelled risk: under 0.12
late 7.7 %, 0.12–0.2 late 10 %, 0.2–0.3 late 27.5 %, over 0.3 late 28 %, so the model's
term for them is about right. A world has 61 experienced drivers (38–84), 7.8 % of its
roster. They stay declined; the switch `premiumCap` is kept.

**The bulk batch needs no rule of its own.** A week has one batch of 30 cars or more from
one yard; e21 wins 17 of its 35 cars. Those orders are late 6.5 % of the time where the
model says 3.6 % (batches of 2–29 cars: 2.7–3.1 % against 3.1–3.3 %), and earn €41 each
against €40 for single requests. The lateness does not rise with the cars already won
from the batch (6.4 % for the first five, 6.3 %, 5.5 %, then 9.9 % from the 21st on, 91
orders) nor with the cars won per driver living near the yard. It is half a late order a
week.

**The cost between 1.1 and 1.25.** On e21, first 24 worlds: 1.15 gives −€195 (s.e. €157),
22 more orders, on time −0.3 points. (1.1 on e19: +€122 with the worst week at 91.5 %.)

## Midnight screens on e21 (first 24 worlds each, paired; asked for: every small gain)

From now on a setting is screened on the first 24 worlds (about five minutes) and only a
candidate engine is judged on all 48.

**What a late car costs** (48 weeks of e21, 439 orders a week): on time 424 a week,
price €97, fee €56, €41 left. Up to two hours late (10 % back): 1.1 a week, €33 left. Two
to 24 hours (25 % back): 8.8 a week, €14 left. Over a day (50 % back): 4.5 a week, −€32.
Over two business days (all back): 0.6 a week, −€106. Refunds are €671 a week, 3.9 % of
the margin; never delivered is one car in fifty weeks.

| Setting | Margin | On time | Worst week | Verdict |
|---|---:|---:|---:|---|
| Cost 1.18 instead of 1.2 (`costMult`) | +€101 (s.e. €137) | +0.1 | 94.2 % against 91.9 % | For the stack. |
| Cost 1.22 | −€38 (s.e. €159) | +0.7 | 95.1 % | Safer, not dearer; not adopted. |
| Risk in list prices 0.25 / 0.75 instead of 0.5 (`riskCost`), 48 worlds | −€46 (s.e. €99) / −€57 (s.e. €121) | 0.0 | 92.4 % / 92.5 % | 0.5 stays. |
| Belief width per dealer 0.055 (`beliefScale`), 36 worlds | +€52 (s.e. €113) | −0.2 | 93.1 % | Undecided. |
| A big dealer's short cars that can arrive today cost 0.9× while its trust can grow (`leadCost`, new; dealers with a fifth of the requests, drives up to three hours) | −€35 (s.e. €177) | +0.3 | 93.9 % | No gain; at 0.8: −€53 (s.e. €155). |
| Dealers with under 7 % of the requests cost 1.05× (`minorCost`, new) | −€339 (s.e. €176) | +0.3 | 93.6 % | Rejected: 19 orders fewer, and they earn €34 each. |
| Every Friday quote 3 % higher (`fridayMarkup`, new) | −€51 (s.e. €54) | +0.2 | 92.3 % | Rejected: nine orders fewer. |
| A second quote three minutes before the deadline when the price has moved by 2 % (`requoteLead`, new) | +€9 (s.e. €178) | +0.1 | 93.1 % | 181 of 838 quotes are sent again and nothing changes; with 1 %: +€27 (s.e. €181). Switch kept. |
| **Friday's orders cost 0.9× (`fridayCost`)** | **+€76 (s.e. €51, better in 19 of 24)** | −0.2 | 91.8 % | **For the stack.** 83 orders on Friday instead of 71, late 4.0 % instead of 3.0 %. After Friday no request comes, and Saturday and the next Monday still deliver: of Friday's 70 orders, 15 arrive on Friday, 15 on Saturday and 38 on Monday. |
| Friday's orders cost 0.8× | −€60 (s.e. €81) | −0.3 | 91.1 % | Too far: 91 orders. |

A change that acts on Friday alone leaves Monday to Thursday as they were, so its paired
difference has a third of the usual noise.

**Friday, tuned** (first 24 worlds, paired with e21; `riskFriday` is the factor on the risk
cap on Fridays):

| Friday cost | Friday risk cap | Margin | On time | Worst week | Orders |
|---|---|---:|---:|---:|---:|
| 0.95 | × 1 | +€63 (s.e. €38) | 0.0 | 92.2 % | +6 |
| 0.9 | × 1 | +€76 (s.e. €51) | −0.2 | 91.8 % | +12 |
| 0.85 | × 1 | +€20 (s.e. €68) | −0.2 | 90.8 % | +16 |
| 1 | × 1.25 | +€35 (s.e. €54) | −0.1 | 92.9 % | +1 |
| **0.95** | **× 1.25** | **+€119 (s.e. €45, better in 18)** | **−0.2** | **92.5 %** | **+7** |
| 0.9 | × 1.25 | +€114 (s.e. €49) | −0.3 | 91.8 % | +13 |
| 0.9 | × 1.5 | +€152 (s.e. €61) | −0.5 | 90.8 % | +13 |
| 0.9 | × 2 | +€146 (s.e. €54) | −0.5 | 91.6 % | +14 |

**e22.** The Friday setting on all 48 worlds, paired with e21: **+€86 a week (s.e. €30,
better in 32 of 48)**; +€119 on the 24 worlds it was tuned on and +€52 on the 24 it was
not; on time −0.15 points; worst week 92.5 % against 91.9 %. In a world joined mid-week
the Friday cost is not applied (such a world already has its own lower cost): on eight
worlds joined on Thursday €5,400 against €5,421, on time 98.1 % against 97.6 %. A replay
of a swept week through the built defaults: the same decisions. Commit `d890d14`, tag
`engine-e22`.

**Friday's fee creep over the weekend is needed.** Friday's orders are booked at 0.95 of
the typical fee against 0.89–0.92 on the other days, after eight hours instead of two to
four: they wait through the weekend while the hold fee creeps up. Limiting the creep for
gigs due on the Monday (`weekendHold`, new), on e22 and 24 worlds: to 1.05× −€9 (s.e.
€35), on time −0.3 points; to 1.0× +€4 (s.e. €35), −0.3; to 1.15× −€25 (s.e. €49), −0.1;
the worst week falls from 92.5 % to 90.6–90.9 %. Switch kept, off.

**A stack of screened gains must be judged on worlds it was not screened on.** Cost 1.18
(+€101 on the first 24) stacked with the Friday setting: +€199 on the first 24 and −€162
on the fresh 24, +€18 on all 48 (s.e. €118). The Friday setting alone held on both; the
cost did not. Only the Friday setting went into e22.

**The same by deadline instead of by weekday does not pay.** A dealer's trust halves when
a late car arrives, so an order due after the week's last request (Friday 18:00) cannot
lose a price by being late: Thursday's next-day orders and all of Friday's. With the risk
cap × 1.25 for such options and no trust counted at stake for them (`endRisk`,
`trustByDeadline`, new): +€11 (s.e. €106), on time −0.4 points; with Friday's cost at 0.9
as well +€9 (s.e. €134) and −0.6 points; cap × 1.5: +€105 (s.e. €122) and −0.9 points.
Thursday's orders are due on Friday evening and need Friday's drivers, who are busy;
Friday's own orders have Saturday and the Monday.

**Leaving a car undelivered is never the cheaper end.** The rules: no price and a penalty
of twice the price, and it counts as late; a car delivered more than two business days
late costs its driver's fee (€106 on average). The engine holds a late car's fee at the
typical fee and raises it only near the end of the world, never above twice the car's
price, which is where delivering stops being the cheaper of the two.

**What predicts the room above our price: nothing but the level, and Friday.** 300,535
real quotes of 456 weeks: win against the quote's distance from the engine's level alone,
leaving whole worlds out: AUC 0.7429, log-loss 0.5794; with 31 more inputs (option set,
lane length, list price, fee over list, hour, weekday, private address, electric, note,
trust, dealer size, batch size, ready later): 0.7434 and 0.5786. The only input of size is
Friday (worth 3.9 % on the level); the rest are under 1.5 % (hops under 30 km −1.5 %,
after 15:00 +1.2 %, before 10:00 −0.9 %). The 11–12 % between our price and the highest
price a request would have taken is the request's own noise, not something to predict.
Friday's 3.9 % could not be collected as a markup (row above).

## Full test e20 / e21 / e22 from their tags, and the Sunday-night ideas (2026-11-15, 00:50–)

**The three engines, each built from its tag, on all 48 worlds.** Every world of every engine is the same to the cent
as the earlier run made with override flags, so the tags are what was measured before.

| Engine | Margin a week | Median | On time | Worst week | Orders | Fee over typical |
|---|---|---|---|---|---|---|
| e20 | €16,974 | €16,787 | 96.7 % | 92.3 % | 437 | 0.944 |
| e21 | €17,327 | €17,172 | 96.6 % | 91.9 % | 439 | 0.938 |
| e22 | €17,413 | €17,236 | 96.4 % | 92.5 % | 446 | 0.939 |

Paired on the same world: e21 − e20 = +€354 (s.e. 107, better in 35 of 48); e22 − e21 =
+€86 (s.e. 30, better in 32); e22 − e20 = +€439 (s.e. 104, better in 36), on time −0.2
points, no week under 90 %. Worlds under 720 drivers: e22 15,753 at 94.9 %; 720 and more:
18,096 at 97.1 %. The eight Thursday-start worlds were left out to free the game server.

**Screens on e22** (paired against e22 on all 48 worlds unless a row says otherwise; the
game server played a week in about 35 seconds after 01:35, so 48 worlds took four minutes).

| Change (switch) | Margin against e22 | What moved | Verdict |
|---|---|---|---|
| A chain that its driver gives back gets a new attempt (`chainRecoverMin` 90), and a chain made at night counts from 07:00 (`chainNight`) | +€22 (s.e. 77) | Driver fees −€146 (s.e. 67), chains booked +5.5 a week, orders −0.6 (s.e. 0.9) | See below. |
| The same with cars won in one step paired at once (`chainBatch`) | +€116 (s.e. 101) | 7.8 pairs a week made in the step, chains booked +0.9 only, chain discounts on requests −5.9 a week | The pairing itself adds nothing: off. |
| `chainBatch` alone | +€48 (s.e. 98) | as above | Off. |
| Price war watch (`war` 0.8, with it answers judged against the current level) | −€59 (s.e. 92) | Revenue +€169, fees +€244, orders +1.8 | Off. |
| Answers judged against the current level only (`beliefFresh`) | −€69 (s.e. 90) | — | Off. |
| A quote without the fastest option is not priced under a 62 % chance (`partialCap` 0.62) | −€140 (s.e. 94) | Orders −6.6 (s.e. 1.2), revenue −€640 | Off: the cap seen in the logs is not what a higher price meets. |

**Chains that a driver gives back.** Of 107 chains booked a week, 16.8 are given back (a
third of them in the minute after midnight). The engine took 13 of them apart in the same
minute, because the chain's 90-minute attempt had ended hours before; the cars then cost
1.02 times their typical fees against 0.79 for the chain, €303 a week more, with €84 of
refunds. A new attempt at night is wasted: of the chain attempts posted between 00:00 and
03:00 four in a hundred were booked, against six in ten of those posted from 08:00 on
(drivers open the app from about 06:00 to 22:00). With both switches 15.8 chains a week
get a new attempt and 73 % of them are booked again, at 0.81 of the typical fee.

**The price war day.** Found in 44 of 48 weeks from the engine's own level (down 0.20 that
day; Monday 8, Tuesday 6, Wednesday 13, Thursday 7, Friday 10). The day after is priced as
any other morning (+6 points over the expected win chance, as on the days before): the
day's factor takes the war, the prices that hold all week do not move. The war morning
itself is slow: the level is 7 % down at 08:30, 16 % at 09:30 and 22 % only at 10:30, and
until then one quote in five wins instead of one in three. The watch (two cases for the
day, odds from the answers, one chance in the working days left before any answer) does
price lower earlier (wins at 09:00–09:30 up from 20 % to 27 %), but the orders it adds are
sold a fifth cheaper and cost their fees: no gain.

**Quotes without the request's fastest option** (the fast option was too risky): they won
41 % where the model expected 54 %, and about 50 % however low the price. Read as a cap,
a floor on the price should have paid; it lost 6.6 orders a week. Dealers that are offered
a slower option only do answer to its price.

## e23: fee clocks that start over (2026-11-15, 02:05–03:00)

**What was wrong.** A gig's fee creeps up while it waits (from the typical fee to 1.25
times it), and the world's clock went on while a driver held the gig. Of the 39 single-car
gigs a week that a driver gave back, 30 came back as the same gig: first booked at 0.93 of
the typical fee, shown at 1.09 when they returned, taken again at 1.11, in the day within
four minutes. That was €12 a gig, €362 a week, paid for nothing. The handbook says why the
taker is so quick: a gig that comes back after a drop notifies the drivers nearby again.

**What e23 does at level 8** (each a switch):

| Switch | What it does |
|---|---|
| `dropFreshMin` 360 | A single order's gig that its driver gives back with six hours of room is taken out and posted again from the opening fee. 27.5 gigs a week; their next booking is at 0.96 of the typical fee (first booking 0.91), after 64 minutes in the day. |
| `chainRecoverMin` 90 | A chain that is given back gets a new attempt instead of being taken apart on the spot (see the section above). |
| `nightClock`, `chainNight` | A gig or chain made between 22:00 and 07:00 starts its clock (a chain also its attempt, with an hour more) at 07:00. |
| `holdAwake` | The slow rise of a waiting gig's fee counts 07:00 to 22:00 only. |
| `raceSafe` | A post refused because a driver took the car's earlier gig in the same minute no longer marks the car for a post without windows (the tight mode at the top fee). |

**Measured**, paired against e22 on 96 worlds (the 48 and 48 further ones, `v1`–`v48`),
with override flags before the tag:

| Setting | Margin against e22 | Better in | On time | Driver fees |
|---|---|---|---|---|
| Chain recovery and `chainNight` alone | +€22 (s.e. 77), 48 worlds | 26 of 48 | −0.0 | −€146 (s.e. 67) |
| + `dropFreshMin`, `nightClock` | **+€411 (s.e. 86)** | 66 of 96 | +0.22 | −€164 on the first 48 (fee over typical 0.939 → 0.931) |
| + `holdAwake` | **+€515 (s.e. 78)** | 74 of 96 | +0.23 | — |

`holdAwake` alone is +€105 over the row before it (s.e. 60, better in 52 of 96). On the 48
benchmark worlds the last row is €17,975 a week at 96.65 % on time, weakest week 92.0 %,
all 48 ranked; on the eight original bench worlds €16,530 (e22 €16,250).

**One week under 90 %.** On the 48 further worlds one week (`v26`, 685 drivers) ends
unranked: 88.9 % on time with e22, 89.3–89.8 % with the e23 settings. Its Tuesday brought
142 orders, and the orders won from Tuesday to Thursday were late 12–14 % of the time, the
"safe" ones (modelled risk under 2 %) 6 %. The on-time outlook saw nothing until Friday:
the risk cap stayed at 0.12 all Wednesday and Thursday. A sign exists three days earlier:
the share of orders won three hours ago or longer that never had a driver is 3 % in an
ordinary week and was 21 % in that week on Tuesday evening (correlation with the week's
late share 0.5–0.6 from Tuesday evening on). A guard that shrinks the risk cap on it
(`stuckFrom` 0.08, `stuckFull` 0.14, `stuckMin` 0.3) did trip there on Tuesday and
Wednesday, but the week still ended at 89.8 %, and on all 96 worlds it cost €20 (s.e. 33):
off. Declining risky requests is not enough in such a week; what is late there is late
across the board.

**Ranking round "Sunday 00:00", level 8** (played 02:54–02:56 with e23's logic, build
`935333aadcb8`, a few minutes before the tag): 18,999 / 19,825 / 18,815 / 20,398 / 18,159,
all ranked, total **€96,195.13, 97.0 % on time, 2nd of 240** (first juhus €102,581.93,
third seekers €94,577.74).

## After e23: the tag on the 48 worlds, and the search for e24 (2026-11-15, 03:05–)

**The tagged e23** (`6da13fe70519`) on the 48 benchmark worlds: €18,002 a week, 96.71 % on
time, weakest week 92.0 %, 48 of 48 ranked; paired against e22 +€589 (s.e. 105, better in
41). On the eight original bench worlds €16,508 (e22 €16,250). The run of the same settings
before the tag (section above) gave €17,975 and €16,530: e23 is the one tag whose bench is
not its settings run to the cent.

**Screens on the tagged e23** (48 worlds each, paired against it; a run takes four minutes):

| Change (switch) | Margin against e23 | What moved | Verdict |
|---|---|---|---|
| Gigs that waited overnight start their clock over at 07:00 (`morningFreshMin` 360) | −€151 (s.e. 85), better in 16 | Fees −€184, revenue −€266, refunds +€69 | Off. Without `holdAwake` it had looked like +€122. |
| A returned gig keeps its place and is repriced down to the opening fee (`returnReprice`); the rules allow a reprice to lower the fee (RULES.md line 70) | −€257 (s.e. 83), better in 14 | Fees −€117, revenue −€304, refunds +€70, orders −2.2 | Off. The gig is re-booked within minutes at 0.80–0.90 by the drivers its return notifies, yet the orders with a give-back end no cheaper (0.977 of the typical fee against 0.970) and late more often (8.3 % against 7.2 %). |
| Slow rise to 1.15 instead of 1.25 (`holdMax`) | −€258 (s.e. 95) | Orders −3.6, on time −0.5 | 1.25 stays. |
| Fresh clock for a returned gig from three hours of room (`dropFreshMin` 180) | −€130 (s.e. 55) | — | 360 stays. |
| Fee steps of 0.017 every 5 minutes (`stepMin` 5, `feeStep` 0.0167) | −€189 (s.e. 129) | — | Off. The same clock run five minutes early. |
| The guard counts a fresh order by its quoted risk for three hours, and starts from 30 orders with one late (`guardGraceMin` 180) | +€50 (s.e. 84), better in 28 | Monday orders 96.5 → 99.2, late cars +1.1, refunds +€52 | Undecided: the artefact is real (Monday's cap was 0.06–0.09 until the afternoon with nothing late) and small. |
| The same with a Monday cap of 1.3 times (`riskMonday`) | −€50 (s.e. 132) | On time −0.34 | Off. |
| The fee clock anchored on the lane's fee when the request's typical fee is above it (`refCap` 1.04) | +€80 (s.e. 113), better in 28 | **Driver fees −€616 (s.e. 77)**, revenue −€490, orders −3.8 (s.e. 1.1), declined as too risky +2.7 | See below. |

**Give-backs** (tagged e23, 48 weeks). 12.6 % of single bookings and 15.3 % of chain
bookings are given back. The share does not depend on the fee the gig was booked at (13.7 %
at the opening fee, 14.3 % up to the typical fee) but on how long the driver holds it:
2.8 % when the last pickup is within six hours of the booking, 11 % within a day, 15.6 %
beyond. An order with one give-back pays 0.970 of its typical fee against 0.887 and is late
7.2 % of the time against 2.5 %; with two or more 1.07 and 14.6 %. Posting a car that is
ready tomorrow only tomorrow morning is no answer: 94 % of such cars are booked on the day
they are won, within 22 minutes, at 0.86 of the typical fee and 1.9 % late.

**The typical fee carries noise, and the fee clock pays for it** (measured on one set of
worlds and confirmed on another). The typical fee of a request is the
lane's fee times one factor per world (the worlds differ by 0.4 % only) plus a noise of
13 % for each request. Drivers know nothing of the noise. Orders whose typical fee was
12–16 % above the lane's paid 1.00 times the lane's fee, those at the lane's level 0.91,
those 16 % under it 0.80; no more of them were booked at any given fee. Capping the fee
base at 1.04 times the lane's fee (589 lanes seen 20 times or more in 96 weeks, the
world's factor learned from the first 20 requests) takes €616 a week off the driver fees
of the same orders. In the bench the engine then won 3.8 fewer orders a week (2.7 more
requests declined as too risky, and a win share 0.3 points lower at prices 0.2 % lower):
cars that wait a little longer for a cheaper driver raise the modelled risk of the next
requests near them.

## e24: the lane's fee in place of the noise of a request's typical fee (2026-11-15, 03:30–04:30)

**The mechanism.** The handbook says the typical fee is the lane's cost times one factor
per world "and adds a noise for each request". Fitted on 96 weeks, the lane table (880
lanes, 589 seen 20 times or more) leaves a residual of 13 % s.d.; the worlds' factors
differ by 0.4 % only. The engine used the noisy number three times: the fee clock opened
at 0.8 of it, the quote's cost was 1.2 times it, and the lateness model read it (its two
fee terms, −1.07 and −2.25, had learned that a high typical fee means an early booking,
which was the clock's own doing).

By the residual, out of sample (table from 48 worlds applied to the other 48):

| Typical fee against the lane's | Requests a week | Won | Fee paid over the lane's fee | Price over the lane's fee | Refunded |
|---|---|---|---|---|---|
| more than 10 % under | 165 | 50.9 % | 0.817 | 1.457 | 4.9 % |
| 4–10 % under | 175 | 50.0 % | 0.861 | 1.519 | 3.4 % |
| within 4 % | 290 | 46.9 % | 0.897 | 1.561 | 2.3 % |
| 4–10 % over | 171 | 42.0 % | 0.943 | 1.614 | 1.8 % |
| more than 10 % over | 194 | 31.5 % | 1.044 | 1.711 | 2.0 % |

**The switches** (`laneref.go`, table in `laneref_table.go`): `refCap` caps the typical
fee at that multiple of the lane's fee (times the world's factor, the median residual of
the requests seen, from the twentieth on) for the fee clock; `refCapPrice` for the quote's
cost; `refCapRisk` for the lateness model; `refCapClock` gives the clock a cap of its own.
A typical fee under the lane's is left alone (those orders pay 0.80–0.86 of the lane's fee).

**Measured**, 48 bench worlds, paired against the tagged e23:

| Setting | Margin | Orders | On time | Weakest week | What moved |
|---|---|---|---|---|---|
| Clock only, cap 1.04 | +€80 (s.e. 113) | −3.8 | −0.22 | 92.1 % | Fees −€616, revenue −€490 |
| Clock only, cap 1.00 | +€279 (s.e. 112) | −4.7 | −0.08 | 92.7 % | Fees −€979, revenue −€651 |
| Clock and cost, cap 1.04 | +€646 (s.e. 127) | +23.9 | −0.49 | 92.4 % | Revenue +€2,333, fees +€1,518, refunds +€170 |
| Clock and cost, cap 1.00 | +€713 (s.e. 130) | +31.9 | −0.84 | 90.6 % | Refunds +€300 |
| **Clock, cost and lateness model, cap 1.04** | **+€558 (s.e. 115)** | +19.4 | −0.14 | 92.1 % | Revenue +€1,605, fees +€967, refunds +€80 |
| Clock, cost and lateness model, cap 1.00 | +€558 (s.e. 137) | +24.6 | −0.45 | 92.4 % | — |

On the 48 held-out worlds (`v1`–`v48`, against e23's settings there) the fifth row is
+€694 (s.e. 96, better in 42) and the third +€608 (s.e. 121). On all 96: third row +€640
(s.e. 89, on time −0.47, `v26` at 88.2 %), **fifth row +€639 (s.e. 75, better in 78, on
time −0.08, `v26` at 90.7 %, no week under 90 %)**. The lateness model on the capped fee
costs nothing in margin and gives the on-time share back: it stops reading a noisy-high
fee as an early booking now that the clock no longer makes it one. The 96-world differences
are against e23's settings run, which earns €27 a week less on the 48 bench worlds than the
tagged e23 of the table above, so they are not the mean of the two halves.

**e24** = e23 + `refCap` 1.04, `refCapPrice`, `refCapRisk`. Commit `04e5c96`, tag
`engine-e24`, hash `1db09c594107`; the tag build replays `n2` and `v26` of the override
runs to the cent. On the 48 bench worlds €18,560 a week, 96.57 % on time, weakest week
92.1 %, all ranked; eight original worlds €17,333; all 96 worlds €18,547, 96.80 %, all 96
ranked. Against e22 on the 96 worlds: +€1,155 (s.e. 89, better in 90).

**Also measured on e23 tonight and left off:** chain fee share 0.85 (−€134, s.e. 113,
5.5 fewer orders).

**Where the ideas came from.** A review from eight angles (drivers, dealers, chains,
leaks, leaders, calendar, rules, on-time); every idea was measured again on other worlds
before it was kept. 25 ideas, 14 kept, 3 marked to build, 5 killed. The lane's fee came
from the chains angle; the Monday guard artefact turned up from three of the angles. Killed
with numbers: a fee that rises with the drivers who looked, deadline pressure on the eve
for long hauls (a rescue costs €107–114 of fee against €93–96 of refund), a risk cut for
short hops, finer fee steps. The leaders' lead on the public feed is on Monday morning
(38 orders by noon against our 24) and in deliveries on the last Monday; nothing in the
feed says how.

## After e24: the tag's own bench, a live rehearsal, and lanes where drivers book at once (2026-11-15, 04:30–)

**The tagged e24** (`1db09c594107`) on the 48 benchmark worlds, label `full-e24`: €18,559.54
a week, 96.57 % on time, weakest week 92.1 %, 48 of 48 ranked, 468 orders; identical to
the cent to the override run on all 48 worlds. Paired against the tagged e23 +€558 (s.e.
115, better in 36).

**Live rehearsal with a restart** (`hc rehearse -seed t7 -step 4 -tick 100ms`, the tag
build): stopped on Wednesday 14:56 and joined again with 5,909 past events, then played
to the end: €17,067.93, 448 orders, 96.4 % on time, nothing undelivered, no error (step
mode on the same world: €17,223.72, 453 orders, 97.1 %). The lane-fee cap is rebuilt from
the replayed requests after the join (358 requests capped over the week).

**Riders on e24** (48 worlds, paired against `full-e24`):

| Change | Margin against e24 | What moved | Verdict |
|---|---|---|---|
| The fee clock's own cap at 1.00 (`refCapClock`) | +€65 (s.e. 123) | Orders −5.4 | Off. At 0.96: −€316 on 18 worlds. |
| Guard grace (`guardGraceMin` 180) | +€48 on 30 worlds (s.e. 154) | On time −0.4 | Off. |
| Lower clock caps on lanes where drivers book readily (`easyCap` 0.87, `midCap` 0.94) | +€31 (s.e. 121) | **Fees −€891 (s.e. 123)**, late cars −0.0, orders −6.9 (declined as too risky +9.3, fast options dropped +11.4) | See below. |
| The same with the count of orders without a driver scaled to 0.9 for the lateness model (`openScale`) | +€166 (s.e. 119) | Orders −3.6, late cars +0.9, weakest week 90.6 % | Off: the model's reaction was right, the orders it lets back in are late. |
| The same caps with the quote's cost on easy lanes capped at 0.95 (`easyPrice`) | **+€217 (s.e. 116)**, better in 28 | Fees −€568, orders +2.8, late cars +0.3, on time −0.08, weakest week 92.4 % | The lead for e25; held out below: +€188 (s.e. 76) on 96 worlds. |
| That with `openScale` 0.9 | +€241 (s.e. 140) | On time −0.32, weakest week 90.8 % | Off. |

**Lanes where drivers book at once.** The noise of the typical fee is an experiment on the
fee clock: it opened lower or higher on the same lane for no reason a driver knows. By how
readily a lane's orders are booked (share booked within ten minutes of the win, measured on
the other half of the worlds, on orders whose typical fee was within 6 % of the lane's):

| Lane | Typical fee against the lane's | Paid, of the lane's fee | Late | Median wait |
|---|---|---|---|---|
| Easy (34 % or more booked within ten minutes) | 10 % or more under | 0.768 | 1.5 % | 31 min |
| | within 4 % | 0.877 | 1.6 % | 17 min |
| | more than 10 % over | 0.998 | 1.0 % | 5 min |
| Middle | 10 % or more under | 0.804 | 2.1 % | 54 min |
| | within 4 % | 0.903 | 1.6 % | 35 min |
| Hard (under 22 %) | 10 % or more under | 0.832 | 6.7 % | 112 min |
| | within 4 % | 0.907 | 3.3 % | 55 min |
| | 4–10 % over | 0.938 | 2.2 % | 36 min |

On an easy lane a clock that opens lower is booked at a fee as much lower and no later; on
a hard lane it doubles the lateness. That is why every opening fee tried for all lanes at
once (0.7, 0.75, 0.9) lost. The table `laneQuick` holds 110 lanes (40 or more such orders
in 96 weeks, 71 % of the orders): 36 % of those orders are on easy lanes, 43 % on middle
ones, 21 % on hard ones.

With the lower caps the same world holds 10 % more orders without a driver at any moment
(14.96 against 13.54 when a request arrives), and the lateness model's strongest load input
is the log of that count: it adds 6 % to the modelled risk of every request and 12 % to a
same-day option's. Scaling the count back lets those requests in again, and they are late.

**The lead on 96 worlds, and the weeks in worlds with few drivers** (05:00–05:35; against
e24's settings, label `e23rc104pr`):

| Setting | Margin against e24, 96 worlds | On time | Weakest week | Weeks ranked | Weeks with a car not delivered |
|---|---|---|---|---|---|
| e24 | — | 96.80 % | 90.7 % | 96 | 4 |
| `easyCap` 0.87, `midCap` 0.94, `easyPrice` 0.95 | +€188 (s.e. 76), better in 59 | 96.74 % | 89.8 % (`v26`) | 95 | — |
| + the caps off while 10 % of the orders never found a driver (`rescueFrom` 0.10) | +€261 (s.e. 74), better in 61 | 96.81 % | 89.9 % (`v26`) | 95 | 11 |
| + risk × 1.25 in worlds under 720 drivers (`thinRoster` 720, `thinRisk` 1.25) | +€113 (s.e. 78), better in 55 | 96.91 % | 91.5 % | 96 | 11 |
| + risk × 1.5 there | +€17 (s.e. 84), better in 50 | 97.14 % | 92.7 % | 96 | — |

The gain of the easy-lane caps is larger in worlds under 720 drivers (+€439 in those 30
worlds, +€180 in the other 66), and so is the danger: every week under 93 % on time is in
a world under 730 drivers (16 worlds under 680 drivers: 95.2 % on time with e24; 680–720:
96.6 %; 720–800: 97.0 %; over 800: 97.5 %). The rescue switch trips in 41 of 96 weeks at
some moment and does not lift `v26`; the caution in thin worlds does (89.9 % → 91.5 % →
92.7 %), at €473 and €780 a thin week. A day whose wins pass 0.15 per driver happens on 73
of 480 world-days; the 8.9 orders a week won after that line are late 5.5 % of the time
against 3.4 %: too weak a sign for a guard of its own.

With the lower caps 11 of 96 weeks ended with a car not delivered against 4 (€28 a week
of penalties against €8): a late car's fee rose from the lower base. Cars under the
pressure of their deadline, tight or late now go by the general cap (`pressedFee`).

**Ranking round "Sunday 03:00", level 8** (played from the hosted runner with the tagged e24,
05:31–05:32): 17,667 / 17,175 / 12,445 / 22,376 / 18,753, all five ranked, total
**€88,416.32**; the leader before it stood at €85,282.54. Week 5 ended at 94.1 % on time
with one car not delivered.

## e25: lanes where drivers book at once, and worlds with few drivers (2026-11-15, 05:35–05:55)

**e25** = e24 + `easyCap` 0.87, `midCap` 0.94, `easyPrice` 0.95, `rescueFrom` 0.10,
`thinRoster` 720 with `thinRisk` 1.25, and cars under deadline pressure or late on the
general cap. Commit `d101989`, tag `engine-e25`, hash `ef514da1ef79`.

| | e24 | e25 |
|---|---|---|
| 48 bench worlds, a week (the tags' own runs) | €18,559.54 | €18,692.42 |
| On time, weakest week there | 96.57 %, 92.1 % | 96.98 %, 94.8 % |
| All 96 worlds, a week | €18,547 | €18,601 |
| On time, weakest week, weeks ranked | 96.80 %, 90.7 %, 96 | 97.02 %, 90.9 %, 96 |
| Weeks under 93 % on time | 3 | 1 |
| Weeks with a car not delivered | 4 (€8 a week) | 9 (€23 a week) |
| Eight original bench worlds | €17,333 | €17,298 |

Paired against e24: 48 bench worlds +€133 (s.e. 119, better in 29), on time +0.41 points;
all 96 worlds +€53 (s.e. 75, better in 53), on time +0.22 points (s.e. 0.10). In the 30
worlds under 720 drivers the money is the same (+€1, s.e. 154) and on time is +0.69 points
(s.e. 0.18); in the 66 bigger worlds +€77 (s.e. 85) with on time equal. Against e22 on
the 96 worlds: +€1,208 (s.e. 94, better in 88). The tag's 48 weeks are identical to the
cent to the override run (`e25c`).

So e25 is e24's money with fewer weak weeks. The easy-lane caps with the rescue switch earn €261 a
week (€188 without it; mostly in thin worlds) and leave the fragile week under 90 %; the caution in thin worlds
gives most of that money back for the on-time share there.

The cars that stay undelivered are orders that a dealer sent to a far town after the win
(Vantaa to Rovaniemi, 824 km, on an order priced €59): no fee under the engine's limit
moves them, and the penalty of twice the price is cheaper than the trip.

**Live rehearsal of the tag** on `v26` (685 drivers, the fragile week) with a restart:
stopped after Wednesday, joined again at Thursday 01:56 with 15,824 past events, played
to the end: €18,214.53, 419 orders, 91.2 % on time, nothing undelivered, no error.

**Fresh worlds** (`w1`–`w24`, seen by no lane table and no choice; the tags with their
defaults, 05:54–06:01): e23 €19,520 a week at 96.35 % on time; e24 €19,888 at 96.19 %
(+€368 over e23, s.e. 257, better in 15); e25 €20,072 at 96.27 %, weakest week 92.4 %
(+€184 over e24, s.e. 190; +€552 over e23, s.e. 241, better in 17). All 24 weeks ranked
with each engine. The order points the same way outside the
worlds the tables were fitted on; on 24 worlds only e25 over e23 is clear of the noise, the
single steps are not.

**Deeper on easy and middle lanes: the plateau** (06:00–06:20; 48 worlds, paired against
the tagged e25). By the same experiment the profit per request keeps rising as both
anchors fall on an easy lane (0.32 of the lane's fee at the lane's level, 0.40 at 8–16 %
under it, 0.43 beyond, late 1.6–1.8 % throughout), mildly on a middle lane (0.43 → 0.47
→ 0.50) and not on a hard one (0.33, 0.34, 0.30 with 9 % late). Played as a policy it
does not hold, because the orders it adds crowd the same drivers:

| On e25 | Margin | Orders | On time | Late cars |
|---|---|---|---|---|
| Quote cost on easy lanes at 0.85 (`easyPrice`) | +€101 (s.e. 130) | +12.2 | −0.15 | — |
| Both anchors at 0.80 on easy lanes and 0.88 on middle ones | −€149 (s.e. 178) | +22.0 | −0.55 | +3.1 |
| Both at 0.84 and 0.92 | +€95 (s.e. 100) | +22.4 | −0.34 | — |
| Clock as e25, cost at 0.80 and 0.90 | +€99 (s.e. 127) | +29.6 | −0.40 | — |

About €100 for a third of an on-time point, inside the noise each time: e25's settings
stay. A request-level experiment gives the direction of a change, not its size once
every request is treated that way.

**Ranking round "Sunday 06:00"** (the tagged e25; level 8 played 08:05–08:07, levels 7 to 1
from the hosted runner 09:54–10:03; five weeks a level, all 40 ranked; places and the other
teams from the public rankings at 10:22):

| Level | Five weeks | On time | Place among the ranked teams | The leader, or the next team |
|---|---:|---:|---|---|
| 8 | €86,394.08 | 96.9 % | 3 of 51 (second when played) | Juhus €98,178.92, Seekers €87,241.12 |
| 7 | €57,117.53 | 97.6 % | 2 of 13 | eth €57,198.40 |
| 6 | €40,679.53 | 98.6 % | 1 of 13 | eth €34,064.62 |
| 5 | €26,925.72 | 98.8 % | 1 of 2 | Northbound Dispatch €21,846.84 |
| 4 | €46,463.16 | 98.5 % | 2 of 2 | Northbound Dispatch €49,564.02 |
| 3 | €62,829.31 | 96.9 % | 2 of 2 | Northbound Dispatch €67,328.17 |
| 2 | €55,120.72 | 96.8 % | 2 of 2 | Northbound Dispatch €60,858.69 |
| 1 | €57,947.50 | 97.0 % | 2 of 2 | Northbound Dispatch €58,036.95 |

Level 8 week by week: 13,445 / 14,506 / 20,498 / 14,317 / 23,628; the first ended with two
cars not delivered. At levels 1 to 4 one team is ahead, by 0.2 % to 10.4 %: levels 1 to 6
were last tuned at e11.

## The final, Sunday 09:00: e25

Played live from the hosted runner with the tagged e25 (`ef514da1ef79`): **€21,111.65, 493
orders, 97.0 % on time, nothing undelivered: third of 236 teams, 58 of them ranked.**

| Team | Margin | Orders | Revenue | Margin share | On time |
|---|---:|---:|---:|---:|---:|
| Juhus | €22,931 | 474 | €53,035 | 43.2 % | 97.7 % |
| blib_blib | €21,745 | 484 | €49,489 | 43.9 % | 96.1 % |
| e25 | €21,112 | 493 | €50,758 | 41.6 % | 97.0 % |
| Seekers | €19,975 | 432 | €47,913 | 41.7 % | 95.4 % |
| MathNet1 | €19,965 | 512 | €55,310 | 36.1 % | 94.7 % |

€1,819 behind the first, €633 behind the second, €1,137 ahead of the fourth. e25 delivered
the most cars of the first four. Against Juhus the gap is price: €8.93 more revenue a car
at €3.38 more cost (fees and refunds). Against blib_blib it is cost: €2.81 less a car at
€0.71 less revenue.

The runner joined the world at 08:30 and the week ran from 09:00 to 09:34. 33,457 requests
to the game server, every one answered (median 9 ms, slowest 182 ms); all 1,034 price
requests answered inside their 15 minutes, none refused. The server refused 35 of the
engine's actions, each a race with a driver's booking (21 withdrawals of a gig just
booked, 13 posts of a car already in a live gig, one reprice).

**The week in the run log.** 749 drivers (above the 720 under which e25 turns cautious).
1,034 requests, 282, 153 and 79 of them from the three biggest dealers. 212 declined (104
as too risky, 60 premium cars, 23 trade-ins, 9 on summer tyres, 7 with too little time, 5
with shorter hours, 4 where no price pays), 822 quoted, 497 won, 4 of those cancelled by
the dealer.

| Request arrived | Requests | Declined | Won | Of the quoted | Won at × list | Level without trust | Margin | Late cars |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Monday | 219 | 29 | 73 | 38 % | 1.20 | 1.19 | €2,317 | 0 |
| Tuesday | 204 | 44 | 99 | 62 % | 1.38 | 1.39 | €4,023 | 3 |
| Wednesday | 191 | 35 | 122 | 78 % | 1.62 | 1.63 | €6,312 | 2 |
| Thursday | 241 | 78 | 102 | 63 % | 1.46 | 1.40 | €4,344 | 2 |
| Friday | 179 | 26 | 101 | 66 % | 1.45 | 1.34 | €4,116 | 8 |

"Level without trust" is the multiple of the list price at which quotes stopped winning
that day, fitted on the day's answers with each dealer's trust taken out (trust counted
from the deliveries). Trust of the three biggest dealers at 18:00: 0 / 0 / 0 on Monday,
then 18 / 11 / 5, 25 / 25 / 9, 25 / 22 / 18 and 25 / 25 / 23, with one halving among them
(the second biggest, on Wednesday evening). Monday was thin again, as in the test final:
no trust yet, 38 % of the quotes won, 11 % of the week's margin from 21 % of its requests.

**Wednesday looks like the competitor's outage, in the south.** Requests with a pickup within 80
km of Hämeenlinna stopped winning 30 % higher than the rest of that day (40 quoted, 38
won); the circles around Helsinki, Lahti and Porvoo show 27–28 % (81 of 87 quotes won
around Helsinki). The outage watch was off, as decided on e19 (see "After e19"): on
Wednesday morning 62 of 71 quotes won against the 69 % the belief expected, and by the
afternoon its prices had caught up (57 of 81 won, 74 % expected). The price war does not
stand out: without trust the level was lowest on Monday, but that is also the day on which
no dealer has any trust yet, and one week does not separate the two.

**Drivers.** €28,994.87 in fees, €58.81 a car, 0.86 of the cars' typical fees: 0.92 on the
269 gigs with one car, 0.74 of the two fees on the 112 chains (224 of the 493 cars; 254
chain gigs were posted and 124 booked). Half of the cars had their first booking within
63 minutes of the win, 21 % within a quarter of an hour, 5 % after more than a day. 54 of
435 bookings were given back (12 %); after a give-back 34 single gigs started their clock
again and 12 chains were posted again. The lane's fee capped the typical fee of 370
requests.

**Fifteen cars were late**, each by two hours to a day (a quarter of its price back, €651
in all):

| Won | Cars | What happened |
|---|---:|---|
| Friday | 8 | Hauls of 326–794 km for a Monday deadline, six of them to Oulu or Rovaniemi. Six had no driver before the last on-time pickup and were booked on the Monday afternoon, in the night after it or early on the Tuesday, at 1.0–1.37 of the typical fee; two were booked earlier and given back. |
| Tuesday, Thursday | 3 | Booked on the day of the win in a chain, at 0.70–0.82 of the two typical fees, and driven late: twice the driver collected the car the morning after its deadline, once at 18:57 for a 20:00 deadline 153 km away. |
| Wednesday | 2 | Rovaniemi pickups with five days to go. One was first booked on the Saturday and given back half an hour after midnight on the deadline day, one first booked on the deadline day and given back three hours later; both were booked again after the last on-time pickup. |
| Tuesday | 1 | A pickup at a private address whose window opened 95 minutes after the win (19:45–22:45): no driver that evening, and the next window opened a quarter of an hour before the deadline. |
| Thursday | 1 | Ylivieska–Kokkola, 88 km for the next evening, taken at a modelled risk of 0.116 under a cap of 0.12: first booked at 21:15 on the deadline day. |

Friday's long hauls are the one pattern. Of the 29 hauls of 300 km or more won on Friday 8
were late; of the week's other 464 cars, 7. The model had rated the eight at 0.04–0.14
(Friday's cap is 0.15 since e22). On the 96 bench worlds that class is late 8.1 % of the
time (19 cars a week), against 6.1 % for long hauls won earlier in the week and 2.2 % for
shorter cars, so this Friday was far worse than its class; snow was forecast for Kemi,
Oulu and Rovaniemi that day. The eight still earned €155 after fees and refunds (the 21
that were on time €1,319), and a Friday's late car costs no trust that is used again: a
rule against them would have cost margin here and bought 1.6 points of an on-time share
that was not at risk.

For scale, not as evidence: on the 96 bench worlds e25 earns €18,601 a week on 465 orders,
single weeks from €10,719 to €29,435 (s.d. €3,787), and 22 of the 96 reach €21,112 or
more; on the 24 fresh worlds €20,072, 10 of 24 at or above. One week says the engine ran
as benched and nothing about whether e25 beats e24.

**Where a next version would start** (none of it measured): the price against the leader
(€8.93 a car); chains that are booked early and driven after the deadline (3 of the 15
late cars; two of them halved a dealer's trust while requests were still coming); long
hauls north won on a Friday.

## Test final, Saturday 20:00: a full week, live, with e20

Played from the hosted runner: **€17,755, 442 orders, 98.9 % on time, nothing undelivered:
third of 75 ranked teams (234 entered).** 25,646 requests to the game server, all
answered, none of the 1,030 quotes late or refused.

| Team | Margin | Orders | Revenue | Margin share | On time |
|---|---:|---:|---:|---:|---:|
| Juhus | €19,592 | 469 | €48,067 | 40.8 % | 97.7 % |
| MathNet1 | €18,599 | 500 | €52,543 | 35.4 % | 95.8 % |
| e20 | €17,755 | 442 | €43,616 | 40.7 % | 98.9 % |
| source-exp | €17,606 | 485 | €46,601 | 37.8 % | 97.1 % |

Orders won by day, from the public feed: Juhus 110 / 99 / 80 / 102 / 78, e20 79 / 99 / 77 /
99 / 88, MathNet1 87 / 103 / 86 / 124 / 100, source-exp 103 / 111 / 105 / 87 / 79. Juhus's
lead in orders was made on Monday; it also earns 3.9 % more per order on the same margin
share. Wednesday was the price war.

The week in the run log (859 drivers; the three biggest dealers sent 320, 147 and 99 of
the 1,030 requests): prices won at 1.24× the list price on Monday, 1.39× on Tuesday, 1.20×
on the price-war Wednesday, 1.62× on Thursday, 1.55× on Friday. Trust of the three biggest
dealers on the five evenings: 7 / 1 / 3, then 25 / 13 / 4, 25 / 17 / 12, 25 / 25 / 16,
25 / 25 / 15; one halving all week. **Why Monday was thin:** with no trust yet the market
level stood at 1.255× the list price and the cost (1.2 × the typical fee) at 1.117×, so
the price with the best expected gain sat above the level for 58 % of Monday's quotes, and
those won 27 % against 67 % for the rest. Not the guard (12 requests passed as too risky,
as on Tuesday), not the clock.

**What was tried from it** (all paired with e20; 48 worlds = the 24 of every bench and 24
fresh ones, `u1`–`u24`, on which e20 earns €16,881 at 96.3 % on time, worst week 92.3 %):

| Setting | Margin | On time | Verdict |
|---|---:|---:|---|
| Cost 1.05 on the Monday (`firstDayCost`) | +€24 on 48 worlds (s.e. €145) | −0.3 | 20 more orders on Monday (113 against 93) that earn €280 on the day; the big dealers' trust is 4.0 instead of 3.2 on Monday evening and 14.5 instead of 13.8 on Tuesday, and Tuesday's prices are 1.3 % higher. One fresh week fell to 89.5 % on time. Rejected; switch kept. |
| Cost 1.0 on the Monday | +€22 on 24 worlds (s.e. €185) | −0.1 | The same. |
| Cost 1.12 on the Monday | −€20 on 24 worlds (s.e. €194) | −0.3 | The same. |
| Cost 1.1 in worlds with 720 drivers or more (`rosterCost`, `rosterFrom`) | +€8 on 48 worlds (s.e. €119) | −0.4 | On e19 a cost of 1.1 had earned €455 more in such worlds and €880 less in the thin ones; on e20, which already takes more orders, +€332 on the fresh worlds and −€274 on the first 24. Rejected; switch kept. |

**The lane risk model with a bend at 500 km: better calibrated, worse played.** Hauls of
500 km and more are late 8.9 % of the time where the model says 4.2 %; hauls of 300–500 km
3.3 % against 4.4 %; every other length is within a tenth (216,208 orders). Fitted again on
314,537 orders of 802 weeks with two more inputs (500 km or more, and that times the lead
per drive): leaving whole worlds out, AUC 0.794 against 0.791 and log-loss 0.1292 against
0.1297, and the two bands come to 1.09 and 0.97 of the modelled risk. Played with it:
level 8 on the 24 worlds −€263 against e20 (s.e. €186, better in 7), worlds joined on
Thursday €5,194 against €5,206 with on time 97.5 % against 98.3 %, level 7 −€109 (s.e.
€127). The long hauls are late twice as often as the old model thought and still earn
about €74 each; the model that knows it refuses them at the cap. With the cap at 0.16:
+€225 (s.e. €174) and on time −0.5 points; at 0.2: +€324 (s.e. €121) and −0.8 points,
which is the same trade as raising the cap on the old model. Not adopted; the weights are
as they were.

**Dealers differ in the delivery option they take, and a premium on speed still loses.**
Of the orders won with several options quoted (58 % of all), 80 % take the fastest; the
share is a dealer's own, new in every world (the biggest dealer: 51 % to 100 % in nine
worlds), and a count per dealer learns it within the week. Tried on e20: a 4 % premium on
the same-day option for every dealer (`sameDayPremium`): −€195 on 48 worlds (s.e. €108),
ten orders fewer. The same premium on the fastest option only for a dealer that took the
fastest in nine of ten orders with a choice, from eight such orders on (`speedPremium`,
new): −€34 on 48 worlds (s.e. €78); at 8 %: −€94 on 24 (s.e. €151). The dealer does not
pay for speed beyond the list price's own step between the options. Switch kept, off.

**A language model in the quote loop: no.** Measured separately on e20's
weeks (1,637 real quotes held out): asked for the chance that a quote wins, with the day's
answers, the dealer's history and summary tables, the model reaches the log-loss of the
engine's own belief (0.615–0.617 against 0.608) and a blend never beats the engine alone;
with a short context it is worse than a constant (0.727). Median answer 0.3 s, but one
call took 31 s. The same work confirms the belief itself: quotes within 3 % of the level
win 51–54 % on every weekday.

## Ranking round "Saturday 18:00", level 8 only: e20 (played 2026-11-14 19:55–19:58)

Five runs, all ranked: **€16,251 a week, 437 orders, 97.3 % on time: third of 61 ranked
teams at 19:59** (first Juhus €18,889, second mathnet1 €16,491, fourth source-exp €15,966).

| Week | Margin | Orders | On time | Drivers |
|---|---:|---:|---:|---:|
| 1 | €16,260 | 439 | 97.3 % | 824 |
| 2 | €20,017 | 494 | 97.6 % | 810 |
| 3 | €16,045 | 430 | 95.3 % | 705 |
| 4 | €13,237 | 404 | 97.8 % | 933 |
| 5 | €15,699 | 420 | 98.8 % | 782 |

No week under 95 %. Juhus stands alone, a seventh above everyone else; the next seven
teams are within €1,800 of each other.

## Test final, Saturday 14:00 (engine e16, played live 14:00–14:19)

A test final is a level 8 world joined on Thursday 00:00: the first three days ran without
any team (614 requests went to the competitor unanswered), and the world plays from there
at the final's clock. Result: **€2,980, 119 orders, 100 % on time, 15th of 55 ranked**
(217 teams). The first had €4,023 with 158 orders.

| | Orders by Thursday 20:00 | Added on Friday | Final | Price / order | Margin / order | On time |
|---|---:|---:|---:|---:|---:|---:|
| akanegakirru, klmisbest, Juhus, eth | 93–98 | +61–64 | 158–160 | €62–65 | €22–25 | 98–99 % |
| e16 | 54 | +66 | 119 | €68 | €25 | 100 % |

- **The live loop worked.** 16,520 requests, all answered (median 43 ms, slowest 1.2 s, no
  rate-limit refusal); every quote went out within two world minutes of its request; no
  failed visit, nothing late, nothing undelivered.
- **The gap is Thursday.** By 10:00 the leaders had 15–16 orders and e16 ten; by the evening
  about 95 against 54. On Friday e16 won as many as they did.
- **Why:** on Thursday the dealers accepted only low prices (e16 won at 1.19× the list price
  and lost at 1.35×; on Friday afternoon it won at 1.32×). A request that is not quoted costs
  a little goodwill, and three days of them had gone by; goodwill and trust come back with
  each order delivered on time. e16 prices on a cost of 1.2× the typical fee, which is about
  1.1× the list price: in a market at 1.2–1.25× that leaves it bidding around 1.30× and
  winning 27–32 % of its quotes. The leaders bid some 5–8 % lower, won 40 more orders that
  day, and had more trust to sell into on Friday.
- **What it says about the real final:** little. The final is a full week from Monday, with
  no days of unanswered requests before it; that case is what the 24 practice worlds and the
  ranking rounds measure. It does say that e16 leaves orders on the table whenever prices
  are thin and drivers are free (100 % on time with 119 orders).
- `hc run -from 4320` and `hc sweep -from 4320` now play a practice world the same way: the
  world runs to Thursday 00:00 without the system, then the system joins.

Tried on the 24 full weeks after it (e16: €16,127, 96.9 %, worst 94.1 %, about 420 orders):

| Setting | Margin | On time | Worst week | Orders |
|---|---:|---:|---:|---:|
| Cost falls by up to 0.1 while the on-time outlook is above 96.5 % (`loosenCost=0.1`) | €16,142 | 96.5 % | 93.7 % | 434 |
| … by up to 0.2 (`loosenCost=0.2 costMin=1.0`) | €16,137 | 96.5 % | 92.0 % | 448 |
| Starting price level 1.25 instead of 1.32 (`marketStart`) | €15,973 | 96.6 % | 93.8 % | 420 |
| Cost falls by up to 0.15 between 97 % and 99 % outlook, not under 1.05 | €16,087 | 96.6 % | 94.1 % | 431 |

All four are level on margin (within noise) and none is safer, so the full week keeps
e16's settings. They were not rejected for adding no margin alone: a setting that is
neutral on full weeks and helps a world joined mid-week is acceptable, and that is what
the Thursday-start worlds were used to find.

**A fault found by this test final (fixed in e17).** On joining a world with a past, the
engine replays the past's events to rebuild its books. Up to e16 the replay priced every
past request anew and then took the dealer's answer of that time ("lost", since nobody had
quoted) as the answer to that price. In the test final that taught the belief model some
460 losses before the first real quote: it began Thursday believing dealers accept far
less than they do (its world level ended at 0.93× the list price while it was winning at
1.19–1.32×). The real final cannot be hit (it is joined at Monday 00:00 with no past), but
every test final and every restart with a stale state file was. e17 does not price the
past (`replayQuotes=1` restores the old behaviour to reproduce a run).

**Worlds joined on Thursday** (`-from 4320`, eight bench seeds; this path has no replay, so
it shows the cost alone):

| Cost in the joined world | Margin | On time | Orders |
|---|---:|---:|---:|
| 1.2 (e16) | €4,538 (€4,297 on the five seeds all settings share) | 98.2 % | 134 |
| 1.1 | €4,899 on those five (+14 %) | 98.8 % | 162 |
| 1.0 | €4,866 on eight (+7 %); €4,898 on the five | 98.1 % | 183 |
| 1.0 with risk cap 0.2 | €5,070 on eight (+12 %) | 97.9 %, three worlds at 92–93 % | 188 |
| 1.2 with a starting price level of 1.2 | €4,478 on the five (+4 %) | 98.9 % | 135 |

The rich world `b1` loses with a lower cost (€8,365 → €6,779 at 1.0); the others gain.
e17 takes 1.1, and only in a world joined after its first day with no order of ours
(`joinCost`): a full week, and a restart in the middle of one, keep 1.2.

e17 itself on the eight Thursday-start worlds: **€4,970, 97.8 % on time, 161 orders**
(e16: €4,538, 134 orders); a join cost of 1.05 gives €4,973 at 97.2 %, 1.15 €4,789, and
1.1 with a risk cap of 0.16 €4,956. The level 8 bench of full weeks is e16's to the cent
(€14,711.61). Through the live loop, one practice world joined on Thursday (`hc rehearse
-from 4320`, seed `thx1`), both ways: e17 €7,835 with 200 orders at 98.5 % on time; e16's
behaviour (`replayQuotes=1 joinCost=0`) €7,659 with 187 orders at 97.9 %, its world price
level ending at 0.98× the list price against e17's 1.38×. No failed visit and nothing
undelivered in either. Commit `709b1ca`, tag `engine-e17`.

## Ranking round "Saturday 12:00": e16 (played 2026-11-14 12:26–12:35)

All 40 runs ranked. Standings at 12:36, early in the round (28 teams ranked at level 8, 15
at level 7; more will play before 15:00).

| Level | e16 | Place | Next team |
|---|---:|---|---|
| 8 | €19,798 (97.0 % on time, 469 orders, worst week 94.1 %) | 2 of 28 | DaVinci's Night Shift €19,842 (446 orders, 97.8 %); third source-exp €19,134 |
| 7 | **€10,464** (96.7 %, 357 orders, worst week 94.7 %) | **1 of 15** | akanegakirru €10,317 |
| 6 | **€6,030** | **1 of 15** | tenth €4,570 |
| 5 | **€8,242** | **1 of 5** | |
| 4 | **€8,088** | **1 of 5** | |
| 3 | **€13,674** | **1 of 3** | |
| 2 | **€11,073** | **1 of 5** | |
| 1 | **€12,442** | **1 of 3** | |

Level 7 with the cost of 1.2: 357 orders a week (460 with e15 the round before) at €29.3
an order, like the teams next to it. Level 8 is €44 a week behind the first; its last
week was 94.1 % on time with 454 orders, the others 96.8–98.8 %. One car stayed
undelivered in a level 6 week and one in a level 1 week (0.5 % and 0.1 %).

## e16: level 7 on fresh worlds, and the regression check of every level

Level 7 with the two new models on the 16 fresh worlds `t1`–`t16` (first time level 7 is
judged outside the eight bench weeks):

| Cost / risk cap | Margin | On time | Worst week | Orders |
|---|---:|---:|---:|---:|
| 1.1 / 0.15 (e15) | €10,314 | 96.4 % | 92.7 % | 424 |
| **1.2 / 0.15 (e16)** | **€10,551** | **97.1 %** | **95.2 %** | **356** |
| 1.15 / 0.12 | €10,538 | 97.3 % | 93.1 % | 385 |
| 1.2 / 0.10 | €10,410 | 97.9 % | 96.3 % | 346 |

The bench preferred 1.1 (€8,808 against €8,461 for 1.2), the fresh worlds and the ranking
round do not: richer weeks let a low cost win too many orders. 1.2 it is.

Regression check of e16, the bench of every level against e14 (same eight weeks):

| Level | e14 | e16 | |
|---|---:|---:|---|
| 8 | €12,951 (97.2 %, worst 93.5 %) | €14,712 (97.5 %, worst 95.4 %) | +€1,761 |
| 7 | €7,902 (97.7 %, worst 96.2 %) | €8,461 (97.3 %, worst 95.4 %) | +€559 |
| 6 | €6,256.45 | €6,256.45 | identical |
| 5 | €6,063.87 | €6,063.87 | identical |
| 4 | €6,556.08 | €6,556.08 | identical |
| 3 | €14,108.39 | €14,108.39 | identical |
| 2 | €11,186.12 | €11,186.12 | identical |
| 1 | €11,571.50 | €11,571.50 | identical |

Live loop: a full level 8 week through `hc rehearse` with e15 (world `reh3`): €17,013,
449 orders, 96.2 % on time, nothing undelivered, no failed visit; 34 posts were refused as
"already in a live gig" (a chain post racing a booking, handled as before).

Not tried on top of e16 at the time (the sweeps were stopped for the ranking round) and
tried later: more volume only while the on-time outlook is high (`loosenCost`,
see "Test final, Saturday 14:00"), trust investment (see "e19"), a refit of the lane model
(see "Test final, Saturday 20:00"). Not reported in this file: cost by gig, the belief
model at levels 4–6.

## Ranking round "Saturday 00:00": e14 (played 2026-11-14 08:37–08:46)

All 40 runs ranked.

| Level | e14 | Place | Best |
|---|---:|---|---|
| 8 | **€15,526** (95.9 % on time, worst week 94.2 %) | **8 of 63** | freya €17,185 |
| 7 | €8,357 | 6 of 23 | skadi €8,951 |
| 6 | €8,041 | 2 of 22 | Builder €9,106 |
| 5 | €8,865 | 2 of 10 | Builder €9,864 |
| 4 | €7,987 | 2 of 12 | Builder €8,381 |
| 3 | €13,123 | 2 of 12 | Northbound Dispatch €13,127 |
| 2 | €11,750 | 3 of 12 | Builder €11,995 |
| 1 | €11,703 | 2 of 10 | Builder €12,187 |

Level 8 week by week: €12,849 (95.6 % on time), €15,468, €15,166, **€18,583** (94.6 %),
€15,565. The tenth place stood at €15,229 a week.

To look at next: one car stayed undelivered in a week of each of levels 1–4 (−€426 to
−€470 each; level 3 missed first place by €4 a week), two at level 5 (−€88), three at
level 7 (−€181) and one at level 8 (−€72). The cheap ones look like the deliberate
kind (fee capped by the order's worth); the four of about €220 do not, and levels 1–3
share their worlds, so it is probably one request. Level 6 had two "closed" visits
(−€50): the 20-minute closing margin of e13 applies only where notes are read (7–8).

## Where engine e8 stood (before e9)

What the levels isolate:

- **Levels 1–3: driver fees only** (every quote wins at list price). We pay 0.89× the typical
  fee at level 1 and 0.93× at level 2, where drivers start to learn that our fees rise. Half
  of our gigs wait 30–120 minutes until the clock reaches about 0.85×, and a quarter drift
  into the deadline ramp at about 1.16×. Level 3 drops again: combined trade-in gigs.
- **Levels 4–6: quoting; levels 7–8: shortage.** In driver-poor weeks (about 700 drivers) we
  move 0.44–0.51 orders per driver.

Public data on other teams is results only: the practice rankings (margin, on-time share,
order count per team and week) and, during a test final or the final, the live feed
(`hc watch` records it: adds revenue per team every two seconds).

## e14: judged on 24 worlds instead of 8

The eight bench weeks cannot tell settings apart that differ by less than €300–400, and
they turned out to be kind to the engine: e13 on **17 fresh worlds** (`t1`–`t16`, `reh1`)
earns €13,832 a week at 95.1 % on time, and its **worst fresh week is 90.3 % on time**,
a hair above the line below which a week does not count.

Two combinations of ideas that were each "noise" on the bench were therefore played on
24 worlds (the 8 bench weeks and `t1`–`t16`), paired with e13 on the same worlds:

| Setting (level 8) | Mean margin | Against e13 | Better on | On time | Worst week | Ranked |
|---|---:|---:|---:|---:|---:|---:|
| e13 | €13,689 | — | — | 95.6 % | 90.3 % | 24/24 |
| A: lane effects + a learned shift per delivery place (`laneWeight=1 destRate=0.02`) | €14,041 | +€352 (s.e. 156) | 17 of 24 | 95.6 % | **89.7 %** | 23/24 |
| **B: A + cost 1.2× + 1.0 × lateness risk (`costMult=1.2 riskSlope=1`)** | **€14,006** | **+€317 (s.e. 252)** | 12 of 24 | **96.4 %** | **92.7 %** | **24/24** |

B is e14. Its gain is not spread evenly: the three worlds where e13 was weakest on time
(`t2` 90.3 %, `t1` 92.5 %, `t11` 92.6 %) each gain about €3,000 and two points on time;
elsewhere it moves between −€1,569 and +€1,189. In a hard world the risk-priced cost
turns down the orders that go late, and the lane levels find the price on the others.
A alone earns as much but pushed one week under 90 %, which the champion rule forbids.

From now on a level-8 change is judged on these 24 worlds
(`spread-1,n2,n4,n5,b1,b2,b3,b4,t1..t16`), not on the eight.

Two neighbours of e14 on the same 24 worlds, against e14: steeper risk pricing
(`costMult=1.1 riskSlope=2`) −€89 (s.e. 215); a bar on the chain's booking chance
(`chainMinChance=0.45`) +€133 (s.e. 156) with the same on-time share. Neither adopted.

## e11: the car that was never delivered (found by rehearsing the final)

A full week was played through the live loop on a fresh world (`hc rehearse`, seed `reh1`:
€9,958, 286 orders, 95.5 % on time, nothing undelivered, 53 of 84 chains booked) and then
once more in practice mode for comparison. The practice run left **one car undelivered**:

- Helsinki → Rovaniemi, 811 km, a drive of 9 h 09, won on Friday 13:11 for €343 with a
  Monday deadline. The dealer's note said the pickup site is open 10:00–17:00 on weekdays
  (stated: 08:00–20:00). No single day's real hours take the pickup *and* the delivery of a
  nine-hour drive, so no day window existed, and the engine fell back to its ordinary
  gigs, which say nothing about the real hours.
- Nobody booked it in time. A late driver came on Tuesday at 08:00: closed (−€166). The
  next came on Wednesday at 09:59: closed, by one minute (−€150). The world ended with the
  car in the yard: −€687. One order cost **€1,002**, a tenth of that week's margin. The
  eight bench weeks have no failed visit at all, so the bench could not show it.

The fix (in e11, levels 7–8, where such notes are handled at all):

1. A request whose real site hours fit no single day (pickup, drive and delivery with the
   buffer) is declined: day windows cannot protect it.
2. If such a car is in the books anyway (a dealer's change, a restart), every gig for it
   carries a pickup window inside the real hours of the next open day, also when it is
   late and after a failed visit; when the day passes the gig leaves the app and the car
   is posted for the day after.

Checked on the world that showed it (`reh1`, practice mode): €9,000 with the car
undelivered and two failed visits → **€9,966**, nothing undelivered, no failed visit.
On the eight bench weeks the fix is neutral (€12,952 against €12,900).

### e12 (rejected) and e13: the other cars that stayed in the yard, and the closed doors

All 411 level 7–8 runs of e9–e11 (bench and sweeps) were then searched for undelivered
cars and failed visits: 19 runs had an undelivered car, 37 had failed visits (all
"closed", 44 visits, €1,810). Three distinct undelivered cars:

| World | Car | What happened | Verdict |
|---|---|---|---|
| `reh1` | Helsinki → Rovaniemi, real pickup hours 10–17 | Above. | Bug, fixed in e11. |
| `b1` | Hyvinkää → Rovaniemi, 794 km, €336 | Booked on Friday, dropped on Monday 00:15 after three days, late. A late gig (no windows) was booked on Tuesday and the driver collected the car on the world's last day at 11:57: a drive of 8 h 55 to a site that closes at 20:00. On the road when the world ended: −€672. | **Not fixed.** e12 gave late gigs a delivery window that ends when the site closes for the last time in the world. Late gigs then left the app two to three times as often (8 → 21 in week `b1`, which lost €1,497), and the bench came out €84 lower with five of eight weeks worse. Rejected and never committed. Seen once in 411 runs. |
| `b4` | Hyvinkää → Hyvinkää for €33, changed by the dealer on Friday to Ylivieska (479 km) | The fee is capped at twice the order's price when late (€65); nobody drives 479 km for that. Penalty −€65. | Intended: delivering would have cost more than the penalty. |

The failed visits: 40 of the 44 were at a site with shorter real hours **on a gig that
had its day windows**. The pickup window ended at the real closing minute, and the
driver came exactly then (17:00 at a site open until 17:00, 15:30 for "collect before
15:30", 14:00 for "open 8–14"): closed. **e13** ends such a pickup window 20 minutes
before the real closing (day windows and the real-hours windows of e11 alike).

Still open: a driver who announces a late pickup (`driver_late`) for a minute after the
real closing still comes and fails; releasing them is not built.

## Lower levels: what e11 changed (eight bench weeks per level; e13 decides the same below level 7)

| Level | Before | e11 / e13 | On time (worst week) | Orders | What changed |
|---|---:|---:|---|---:|---|
| 1 | €9,280, one week unranked (89.6 %) | **€11,571** | 97.2 % (95.7 %) | 804 | Lead rule. |
| 2 | €6,644 | **€11,186** | 96.9 % (93.4 %) | 804 | Fee clock opens at 0.75× and rises 2 % a step (€9,100), lead rule (€10,858), hold at 0.9× (€11,186). |
| 3 | €9,213, one week unranked | **€14,108** | 98.2 % (97.1 %) | 804 | The level 2 clock, chains (€11,607), trade-ins as two gigs (€11,880), lead rule (€13,789), a second chain attempt per order (€14,108). |
| 4 | €3,874 | **€6,556** | 98.5 % (97.5 %) | 405 | As level 5 (ahead on 8 of 8 weeks). |
| 5 | €3,591 | **€6,064** | 97.8 % (96.6 %) | 388 | Price floor 0.85× and cost 0.92× the typical fee (€4,395 at 0.92/0.92 alone), the level 2 clock, lead rule, chains, trade-ins as two gigs. |
| 6 | €3,819 | **€6,256** | 97.7 % (96.7 %) | 406 | As level 5 (ahead on 8 of 8 weeks). |
| 7 | €6,668 | **€7,902** | 97.7 % (96.2 %) | 267 | Cost 1.22× the typical fee instead of 1.3× (and the chains of e9, which alone give €7,484); e13's closing margin (+€55). |

"Before" is the earlier default on the same eight weeks (level 3: already with the level
2 clock; its older default was not benched). The ranking weeks are richer than the bench weeks at every level. Every e11
number is the level's default setting on the eight bench seeds; a world is the same to
the cent for the same decisions, whichever label the run carried.

- **The lead rule** (`minLeadMin` 180, `minLeadDrive` 1; levels 1–6): an option is declined
  when the minutes from the dealer's answer to the last on-time pickup are fewer than
  180 + 1.0 × the drive. At level 1 the orders won with under 5 hours to the last pickup
  (93 a week) lost €1,150 a week and were late 16–43 % of the time; with a drive over
  300 km even 5–8 hours lost money (44 % late). Levels 7–8 leave this to the risk model.
- **Chains below level 7.** With drivers enough, chains are booked more often and save
  more: at level 3 211 attempts a week, 144 booked, +€2,400 a week. A second attempt for
  an order whose chain found nobody pays at level 3 (+€319, worst week 95.9 % → 97.1 %),
  not at level 8 on the eight weeks (+€114, noise; on 48 worlds it later earned +€354 and went
  into e21). Three cars, a hop of 120 minutes and a fee share of 0.8
  were all worse at level 3 (−€56, −€762, −€466).
- **Trade-ins as two gigs** (`splitTradeIns`): +€274 at level 3 and 1.4 points on time.
- **The hold fee** (`feeMid`): 0.9× is better at level 2 only (+€328); at levels 1 and 3
  it loses (−€959, −€284).
- **Levels 4–6 price a little higher than the model's optimum.** Cost 0.92× beats 0.85×
  (€6,064 against €5,686 at floor 0.85×): 16 % fewer wins, more margin. See the price
  sensitivity above.
- **Level 7:** cost 1.3× €7,484 (224 orders), 1.22× €7,842 (267), 1.15× €7,952 (311, worst
  week 93.8 % on time), 1.0× €6,653 (386 orders, 93.5 % on time, worst week 90.0 %). 1.22×
  and 1.15× earn the same within noise; 1.22× is the default because it is safer. Risk cap
  0.5 at 1.15× is within noise of 0.35 (€8,000, less safe).

