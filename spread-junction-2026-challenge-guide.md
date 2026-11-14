# Spread — Junction 2026 challenge, API, and implementation handoff

**Organizer:** Bahn Express  
**Website:** <https://spread.bahnexpress.com/>  
**Verified:** 14 November 2026, Helsinki time (EET, UTC+02:00)  
**Purpose:** A standalone reference for understanding the challenge and implementing a competitive engine. 

The official **[RULES.md](https://spread.bahnexpress.com/kit/RULES.md)** takes precedence when the handbook or explanatory pages disagree. Use **[OpenAPI](https://spread.bahnexpress.com/openapi.json)** and the current SDK types for exact request and response schemas. Recheck event announcements before the final. All monetary amounts in the simulator are game euros.

## Contents

1. [Challenge and winning conditions](#1-challenge-and-winning-conditions)
2. [Official links and where to get data](#2-official-links-and-where-to-get-data)
3. [Authentication and SDK quick start](#3-authentication-and-sdk-quick-start)
4. [World calendar and delivery options](#4-world-calendar-and-delivery-options)
5. [Scoring and financial consequences](#5-scoring-and-financial-consequences)
6. [Customer pricing, competition, and trust](#6-customer-pricing-competition-and-trust)
7. [Drivers, fees, and direct offers](#7-drivers-fees-and-direct-offers)
8. [Gigs, feasibility, and operational recovery](#8-gigs-feasibility-and-operational-recovery)
9. [Notes, vehicle requirements, and weather](#9-notes-vehicle-requirements-and-weather)
10. [Levels and market shocks](#10-levels-and-market-shocks)
11. [API reference and examples](#11-api-reference-and-examples)
12. [Events and state to maintain](#12-events-and-state-to-maintain)
13. [Practice, exports, and ranking rounds](#13-practice-exports-and-ranking-rounds)
14. [Live final, limits, and recovery](#14-live-final-limits-and-recovery)
15. [Schedule, submission, and judging](#15-schedule-submission-and-judging)
16. [Benchmarks, remaining inputs, and sources](#16-benchmarks-remaining-inputs-and-sources)

## 1. Challenge and winning conditions

Build the system that operates a simulated car transport company in Finland for one week. Dealers request prices; a simulated competitor also quotes. If a dealer accepts your quote, you become responsible for the order. You publish gigs, choose driver fees, and react as freelance drivers book, travel, drop bookings, or encounter problems.

Cars move **on their own wheels**, one car driven by one person. Combining cars into a gig means a sequence of car movements, not carrying several cars on a transporter.

The target is the highest **total gross margin**, subject to delivering at least **90% of won, non-cancelled orders on time**. Every leg of a multi-leg order must arrive on time. Zero won orders does not qualify. Equal margins to the cent share a rank.

The final is one secret **level 8** world. Each team gets its own copy with the same initial requests, drivers, and random draws. Teams do not compete directly for the same drivers or cars. Their decisions change how their own world evolves.

The ten eligible teams with the highest final margins reach the jury. The jury then chooses winners based on elegance, clarity, creativity, and robustness. Margin no longer determines the winner after qualification.

| Term | Meaning |
|---|---|
| World | One simulation instance, including its delivery tail after the main working week. |
| Price request | An opportunity to quote for one car, or two cars in a trade-in. |
| Order | A request your team won; one price can cover two legs. |
| Leg | One car's pickup-to-delivery movement. |
| Gig | An ordered sequence of stops offered to a driver for a fee. |
| List price | Public distance-based customer price reference; not the competitor's actual price. |
| Typical fee | `referenceFeeCents`, a normal-week driver-pay reference; not a guaranteed booking fee or the cheapest driver's cost. |
| Ranking round / season | Five secret practice weeks at a given level, shared across teams. |

Sources: [rules](https://spread.bahnexpress.com/kit/RULES.md), [judging](https://spread.bahnexpress.com/judging).

## 2. Official links and where to get data

| Resource | URL | Use |
|---|---|---|
| Main challenge | <https://spread.bahnexpress.com/> | Overview and organizer benchmarks. |
| Agent entry point | <https://spread.bahnexpress.com/llms.txt> | Links to official machine-readable documentation. |
| Handbook | <https://spread.bahnexpress.com/kit/HANDBOOK.md> | Mechanics, examples, SDK usage, and research guidance. |
| Rules | <https://spread.bahnexpress.com/kit/RULES.md> | Authoritative game rules. |
| API reference | <https://spread.bahnexpress.com/kit/API.md> | HTTP routes, streaming, limits, and error behavior. |
| OpenAPI contract | <https://spread.bahnexpress.com/openapi.json> | OpenAPI 3.1 schemas; API version 0. |
| Human explanation | <https://spread.bahnexpress.com/how> | Plain-language mechanics and final operation. |
| Schedule and judging | <https://spread.bahnexpress.com/judging> | Deadlines, presentation expectations, and final rules. |
| Register | <https://spread.bahnexpress.com/join> | Create a team and obtain its key. |
| Team page | <https://spread.bahnexpress.com/team> | Team worlds, final access, and submission hash. |
| Kit directory | <https://spread.bahnexpress.com/kit/> | Current downloadable SDK packages. |
| TypeScript kit | <https://spread.bahnexpress.com/kit/spread-typescript.tar.gz> | Client, types, starter, and CSV exporter. |
| Python wheel | <https://spread.bahnexpress.com/kit/spread-0.1.0-py3-none-any.whl> | Python client, starter, and exporter. Check the directory for updates. |
| Map API | <https://spread.bahnexpress.com/v1/map> | Public places and road-distance/time matrix. |
| Level 8 practice results | <https://spread.bahnexpress.com/v1/leaderboards/practice?level=8> | Public JSON for current and previous rounds. |
| Leaderboard page | <https://spread.bahnexpress.com/leaderboard> | Human view of rankings and final. |
| Live final leaderboard | <https://spread.bahnexpress.com/v1/leaderboard> | Public server-sent events, not ordinary one-shot JSON. |
| Run viewer | <https://spread.bahnexpress.com/viewer/> | Team run replay, order stories, and financial breakdowns. Uses the team key entered on the site. |
| Big screen | <https://spread.bahnexpress.com/screen/> | Public display interface. |

### Where the actual training data comes from

There is no public download containing all hidden simulator values. Create ordinary practice worlds, run a policy, save events and actions, then request each completed world's reveal. The SDK can export CSV files.

| Available data | Access | Important limitation |
|---|---|---|
| Places and roads | Public `GET /v1/map` | Road times are published; public-transport times are hidden. |
| Requests and quote outcomes | Your world's events; completed reveal | Competitor quotes are never disclosed. |
| Driver roster | Your world's `/drivers` | Home, rating, licence, completed gigs, drops; not their full daily availability or private costs. |
| Driver location reports | Snapshot `driverPositions` and your job events | Information from your own gigs, not universal live GPS. |
| Gig views, bookings, drops, offers | Events; views also in reveal | A view does not establish suitability; an unread offer can expire. |
| True car constraints | Completed reveal for orders you won | Does not reveal hidden truth for all lost requests. |
| Money | `ledger` events and `/score` | Revenue and costs settle over time; early positive margin is not final profit. |
| Cross-team comparison | Public ranking endpoints | Does not expose other teams' private worlds, source code, or hidden seeds. |

Sources: [API](https://spread.bahnexpress.com/kit/API.md), [handbook research section](https://spread.bahnexpress.com/kit/HANDBOOK.md#research).

## 3. Authentication and SDK quick start

Create a team at `/join`. The team shares one key, including its rate limits. The supplied challenge-listing screenshot gave event code **`spread-kmu9z6`**; use the current event code supplied by organizers if it changes.

Authenticated HTTP requests use:

```http
Authorization: Bearer <team key>
Content-Type: application/json
```

Set these environment variables in the runtime:

```bash
export SPREAD_URL='https://spread.bahnexpress.com'
export SPREAD_KEY='REPLACE_WITH_YOUR_TEAM_KEY'
```

Keep the actual key out of committed files and logs. Set `SPREAD_URL` explicitly: the inspected TypeScript client otherwise defaults to a local server.

### TypeScript

Supports Bun or Node.js **22.18+**. The kit itself requires no additional packages.

```bash
curl -fsSLo spread-typescript.tar.gz "$SPREAD_URL/kit/spread-typescript.tar.gz"
tar -xzf spread-typescript.tar.gz
cd typescript
bun starter.ts --level 8 --seed baseline-1
```

With a supported Node runtime:

```bash
node starter.ts --level 8 --seed baseline-1
```

Kit files include `spread.ts`, `types.ts`, `starter.ts`, and `reveal-csv.ts`. Start from the starter's transport and recovery logic, then replace its decisions. The unchanged starter is deliberately financially weak at level 8.

### Python

Requires Python **3.9+**; the package needs no other packages.

```bash
python3 -m venv .venv
. .venv/bin/activate
pip install "$SPREAD_URL/kit/spread-0.1.0-py3-none-any.whl"
spread-starter --level 8 --seed baseline-1
```

### Starter modes

| Option | Meaning |
|---|---|
| `--level 1` through `--level 8` | Choose difficulty; default is 8. |
| `--seed baseline-1` | Reproducible ordinary practice world. |
| `--board` | Consume the next ranking run at the chosen level in the current round. |
| `--final` | Join the currently scheduled live final or test final. |

Do not combine a chosen seed with a board run. Running the starter performs simulation actions; the examples above are commands for your team to execute, not evidence that this report has run them.

Sources: [handbook setup](https://spread.bahnexpress.com/kit/HANDBOOK.md#the-first-15-minutes), [SDK package](https://spread.bahnexpress.com/kit/spread-typescript.tar.gz), event code from the challenge screenshot supplied in this conversation.

## 4. World calendar and delivery options

World time is an integer count of minutes from Monday 00:00. For example, `1440` is Tuesday 00:00. Money is integer euro cents; fractional time or monetary inputs are rejected. Opening-hours fields use minutes within a day, so keep these separate from absolute world minutes.

Approximately 1,000 price requests arrive during the main week. The map contains 30 Finnish towns. Requests arrive Monday 08:00 through Friday 18:00, mostly 09:00–18:00. Saturday has short dealer opening hours and some closed sites. Sunday dealer sites are closed. The following Monday is a business day with no new requests and contains the final deadlines. The simulation continues until Wednesday 22:00 so late deliveries can settle.

| Option | Deadline |
|---|---|
| `same_day` | 20:00 on the car's ready day. Offered only for requests arriving by 15:00 where the drive could finish two hours before the deadline. |
| `next_day` | 20:00 on the next business day; Friday readiness means Monday. |
| `within_3_days` | 20:00 on the third business day after the ready day, capped at the final Monday. |

Private deliveries instead use their booked hour, with a deadline 30 minutes after that hour ends. A private-delivery or trade-in request offers only one option.

Pickup windows start at the stated ready time. Request windows at dealer sites extend to the latest offered deadline; after winning, order windows use the selected option's deadline. Site opening hours apply independently. Private pickups use booked slots.

Each request has `respondBy`, 15 world minutes after arrival. Quotes must reach the server **before** this minute; a quote at the deadline is too late. The dealer decides using the last quote. A later quote replaces the entire previous quote, not just one option.

Sources: [rules](https://spread.bahnexpress.com/kit/RULES.md#the-week), [request rules](https://spread.bahnexpress.com/kit/RULES.md#price-requests), [world overview](https://spread.bahnexpress.com/how#week).

## 5. Scoring and financial consequences

The score is the sum of the account ledger. Optimize total `marginCents`, not revenue, order volume, or margin percentage.

| Outcome or cost | Account effect |
|---|---|
| Completed order | Receives the quoted price when its final leg is delivered. |
| Up to 2 hours late | Refunds 10% of the quoted price. |
| More than 2, up to 24 hours late | Refunds 25%. |
| More than 24 hours, up to two business days late | Refunds 50%. |
| More than two business days late | Refunds the entire price; delivery is still required. |
| Undelivered at the world's end | No customer revenue and a penalty of twice the price. |
| Completed gig | Pays the driver's booked fee. |
| Failed pickup visit | Pays half the failed car's share of the gig fee, minimum €25. |
| Normal booked-driver release, pickup over 24 hours away | Pays 25% of the booked fee. |
| Normal release, pickup less than 24 hours away | Pays 50%; the under-two-hour tier below overrides this. |
| Normal release, pickup less than 2 hours away | Pays 100%. |
| Release after `driver_late`, before departure | No release fee. |
| Premium car driven by an ineligible driver | €150 fine per car. |
| Required EV charging omitted | €100 fine and the documented 90-minute late-arrival consequence. |
| Unexpectedly closed delivery site | Waiting costs €15/hour, capped at two hours; see section 9. |

The prose rules do not state the exact equality boundary for release tiers at precisely two or 24 hours; avoid relying on that boundary without verification.

An order is on time only if **every leg** meets its applicable deadline. An undelivered order counts against the 90% requirement. Dealer-cancelled orders are excluded. Business-day lateness excludes Saturday and Sunday; Friday to Monday is one business day late.

Useful consequence: winning an order creates an obligation. The public action set has no dispatcher action that cancels the order itself. Withdrawing its gig does not remove the obligation or terminal penalty.

Source: [account and operational rules](https://spread.bahnexpress.com/kit/RULES.md#your-account), [OpenAPI action contract](https://spread.bahnexpress.com/openapi.json).

## 6. Customer pricing, competition, and trust

### Quoting

Offer prices for any nonempty subset of the request's available delivery options. Dealers compare the complete menu against their preferences and the competitor. The chance of winning falls as your price rises relative to the alternative, especially above it. Different dealers value delivery speed differently.

At levels 4–8, decline undesirable work with a deliberately high, losing quote. A losing quote costs no goodwill; leaving a request unanswered reduces dealer satisfaction slightly. At levels 1–3, every quote wins the first option at list price regardless of the quoted amount, so decline by not quoting.

### Competitor behavior

The competitor's prices depend on route, day, world-level conditions, and request-specific variation. Part of the route difficulty persists across worlds; other factors are redrawn. Origins and destinations can change price during the week, and one weekly adjustment changes short and long lanes in opposite directions.

The competitor also reacts to your wins. Each morning it can lower prices for dealers whose recent requests you mostly won, and raise prices where it won most. It does not know your drivers, orders, or dispatch timing. Every team has its own independently responding competitor.

Your win/loss observations are noisy demand signals, not disclosures of an exact competitor threshold. A static table learned in another practice world will not fully describe the current week.

### Satisfaction and level 8 trust

Late or undelivered orders reduce a dealer's willingness to pay. On-time work repairs satisfaction gradually. This does not change the request stream: teams still receive the same requests.

Level 8 adds a per-dealer trust count:

- Starts at zero in every world.
- Each on-time order adds one, up to 25.
- Each point raises acceptable prices by 1%.
- A late order halves the count, rounding down.
- Ordinary dissatisfaction and competitor comparison still apply.

A count of 25 dropping to 12 loses 13 percentage points of trust-related price tolerance. This is a consequence of the stated formula, not a guaranteed revenue loss of that exact percentage. Trust is earned after delivery, not merely by winning or booking an order.

Source: [handbook pricing and trust](https://spread.bahnexpress.com/kit/HANDBOOK.md#dealers-compare-prices), [trust rules](https://spread.bahnexpress.com/kit/RULES.md#practice-and-the-final).

## 7. Drivers, fees, and direct offers

### What drivers know and want

Drivers have home towns, qualifications, ratings, and private schedules. Daily work decisions, available hours, maximum desired gigs, acceptable trip lengths, and return-home preferences are not fully visible. A driver only books future-day work when they know they will work that day.

Their cost includes driving, reaching the pickup, returning home or reaching another gig, transport tickets, and job-specific inconvenience. Outbound public-transport tickets bought ahead are cheaper than uncertain same-day return tickets. Evening, weekend, private-address work, plates, charging, and overnight positioning can add cost.

A driver finishing near another pickup, or delivering toward home, can be much cheaper than a typical driver. Public-transport time has a different private cost from time driving. The typical fee is a reference derived from a typical pickup-town driver; it is not a minimum acceptance price.

### Visibility and notifications

Most gig views occur soon after posting. New-gig notifications normally go to drivers whose homes are within 30 km of the first pickup, measured straight-line. Drivers also browse around their current location, but moving a driver away from home does not move their home-notification radius.

Ordinary reposting does not repeatedly notify drivers of the same order. A drop or no-show reopening a gig can trigger a new notification. Fee increases do not notify. Notifications are not read from 23:00 to 07:00.

A `gig_viewed` event identifies the viewer, but curiosity is common. It does not prove availability, suitability, or a particular reservation price.

### Learned waiting and drops

Drivers learn from gigs they viewed and could perform: how much the fee increased from the first fee they saw, and how long booking took. Their beliefs update over time, with older observations fading. They may wait when expecting a raise. The order retains its age and initial viewed-fee history when reposted.

Bookings can be dropped before departure. Holding a booking longer increases drop exposure; barely sufficient pay can cause a driver to return the job after planning it more carefully. A driver who dropped an order needs a higher fee to book it again. Snow can also cause same-day drops.

### Direct offers

| Rule | Limit or meaning |
|---|---|
| Recipient | One named driver for one existing gig. |
| Offer duration | 10 world minutes. |
| Per-driver limit | At most 3 direct offers per driver per day. |
| Per-gig limit | At most 3 simultaneously open direct offers. |
| Night | No reading between 23:00 and 07:00. |
| Acceptance | Arrives within the offer window and books the gig. |
| Decline | Reported after the window. |
| Expiry | Can mean unread, unavailable gig, or a gig that no longer fits the driver's day. |

A driver who reads a direct offer evaluates its fee against their cost and other opportunities, without the public-app incentive to wait for a fee rise. This can help reach a driver who ends one of your gigs far from home. It does not guarantee acceptance or reveal their exact private cost.

Sources: [driver mechanics](https://spread.bahnexpress.com/kit/HANDBOOK.md#drivers-have-homes-and-days), [direct-offer rules](https://spread.bahnexpress.com/kit/RULES.md#direct-offers).

## 8. Gigs, feasibility, and operational recovery

A gig contains a fixed ordered list of pickup and delivery stops, audience requirements, and a fee schedule of **1–48 steps**. Within one schedule, times are ordered and fees cannot decrease. A reprice replaces the entire schedule; the new schedule may start below the previously displayed fee, provided it is internally nondecreasing. Repricing does not notify drivers. Before a schedule's first step begins, its first fee is already shown; a future first step does not hide the gig.

Audience fields are `experienced` and `manualLicence`. Experienced means rating at least **4.8** and at least **50** completed gigs.

### Feasible routes and windows

- A multi-car gig may plan at most **12 hours of road driving**, including travel between the cars. Plates and charging time do not count toward this particular road-time limit, but do matter for timing. A single-car gig has no such 12-hour limit.
- Drivers book only when they can reach every stop in its applicable window using published road minutes plus plates/charging time. Therefore a gig can become unbookable before its nominal window closes.
- Stop windows cannot start before the order's window. A private pickup's custom window must also end within its booked slot.
- At other stops, a custom window can extend beyond the order deadline to permit late delivery; this does not make the delivery on time.
- Default windows use order timing and site hours. When on-time execution is no longer possible, the documented fallback can extend stops without custom windows to the world's end. A private seller's slot never automatically extends this way.
- An open gig expires in the minute after its earliest stop window ends. Expired gigs leave the app; the cars still need service.

### Trade-ins

Put the old car's pickup immediately after delivery of the new car in a combined trade-in gig. They then happen in the same visit, including a late visit; that old-car pickup slot does not separately block booking the combined gig. Split trade-ins that exceed the 12-hour multi-car limit. If the old car is posted separately, its private slot applies normally; an uncollected private car can receive a next-business-day slot through `order_changed`.

### Recovery signals

`lastPickupAt` is the last collection time that can still deliver the remaining order on time. It updates with snow, changed plans, and completed legs. When no on-time pickup exists, it instead represents the earliest possible collection time, so it is not an unconditional feasibility certificate.

`deadline_close` usually arrives three hours before `lastPickupAt` if the order lacks a driver. It can arrive again after a later drop or when the last pickup moves later. Drivers need time to read offers and travel, so a last-minute fee increase may be useless.

`driver_late` means the driver still intends to come and supplies `pickupAt`; release is free before `gig_started`. `no_show` means they will not come and the gig is open again. `gig_dropped` also reopens the gig with its schedule.

### Dealer changes and cancellations

A destination change takes effect in the world even if a driver has booked or started. Extra driving can increase the driver payment. Reassess the deadline and remaining work rather than assuming the original route survives unchanged.

For cancellation, the event reports how affected gigs change:

| Effect | What happened |
|---|---|
| `withdrawn` | An open gig disappeared; other orders' cars in it must be reposted. |
| `released` | A booked gig containing only this order releases the driver under release-fee rules. |
| `shortened` | A booked mixed-order gig continues with other cars; the booked fee remains. |
| `continues` | The driver already left, skips the cancelled car, and receives the full fee. |

Source: [gig rules](https://spread.bahnexpress.com/kit/RULES.md#gigs), [changes and cancellations](https://spread.bahnexpress.com/kit/RULES.md#notes-and-changes-of-plan).

## 9. Notes, vehicle requirements, and weather

Notes are operational input in Finnish, Swedish, or English. They can include abbreviations, typos, multiple languages, corrections, negation, several constraints, or no relevant constraint. The simulator follows their meaning whether the engine reads them correctly or not.

Extract readiness, payment-release time, pickup and delivery hours, tighter deadlines, manual gearbox, missing plates, and summer tyres. A date correction overrides the earlier date; a denied time must not be used.

### Date and time interpretation

- Relative dates are based on the request's arrival day.
- A time without a day refers to that arrival day.
- Today/tomorrow/day-after-tomorrow expressions and common abbreviations count in each supported language.
- A named weekday denotes that weekday **2–6 days after arrival**; notes do not use weekday names for the arrival day or the next day.
- Numeric times use 24-hour interpretation unless `am` or `pm` is specified.
- Written-out hour expressions use the corresponding working hour from 09:00–17:00 under the stated note conventions.
- The special convention for “half three” is 14:30.
- If actual readiness is one or more days later than stated, the order deadline moves by the same days; `order_won` already includes the shift.

### Vehicle and site rules

| Condition | Required handling / consequence |
|---|---|
| Premium vehicle | Experienced audience: rating ≥4.8 and ≥50 completed gigs. Wrong driver costs €150/car. |
| Manual gearbox | Require a manual licence. Wrong licence causes a failed visit. |
| Missing plates | Set `withPlates` on pickup; fetching plates takes 30 minutes. Omission causes a failed visit. |
| EV below 30% charge on a route over 200 km | Set `withCharging`; planned charging takes 40 minutes. Omission incurs the documented lateness and €100 fine. |
| Summer tyres and snow at pickup or delivery | The car cannot be collected during that snow. Forecast snow before the deadline can move its deadline by one day via `order_changed`. |
| Delivery site closed according to published hours | Driver leaves the car; delivery counts at the next opening. No paid waiting. |
| Delivery site unexpectedly closed despite published hours | Driver waits up to two hours at €15/hour, then leaves it; delivery counts when the site opens. |
| Private delivery before booked hour | Driver waits with the car until the slot; this wait is not charged. |
| Failed pickup in a mixed gig | Driver continues with the other cars; the failed leg needs a corrected new gig. |

The strict EV thresholds are **below** 30% and **over** 200 km. Do not automatically add charging time to every EV. Physical driving times vary around map estimates, and snow slows travel.

Source: [note conventions and car-handling rules](https://spread.bahnexpress.com/kit/RULES.md#notes-and-changes-of-plan), [handbook notes](https://spread.bahnexpress.com/kit/HANDBOOK.md#notes-change-what-happens).

## 10. Levels and market shocks

| Level | Added mechanics |
|---|---|
| 1 — Posting | Single-car gigs, one fee. Every quote wins the first option at list price. Dealer cancellations exist. |
| 2 — Market | Fee schedules, driver learning, and booking drops. |
| 3 — Network | Multi-car gigs and direct offers; same underlying world as level 2. |
| 4 — Quotes | Competitor, delivery-option choice, and satisfaction effects. |
| 5 — Full | Notes, changes, failed visits, handling rules, and weather. |
| 6 — Surprises | Competitor outage every week; flu and bulk-order events in some weeks. |
| 7 — Shortage | About a third of normal drivers, higher competitor prices, and all three surprises every week. |
| 8 — Trust | Uneven town-level shortages, still higher competitor prices, trust, and a price-war day. **Final level.** |

### Level 8 conditions

- Total drivers are approximately **26–40% of a normal world's population**; town allocations also vary, and some places lose almost all supply.
- The competitor's world price-level factor is **26–54% above normal list-price baseline**, before route, day, and request effects. This is not a guarantee that every individual quote is in that range.
- The list price and typical fee remain normal-week references.
- One working-day regional outage runs 08:00–18:00. The affected region is a town and places within 80 km straight-line; busy regions are more likely. A backup competitor costs about 30% more than the usual competitor.
- A flu day removes 30% of drivers who otherwise intended to work.
- A dealer group can issue 30–40 cars from one yard together. Those cars still need separate movements.
- On one other working day, a nationwide price war runs 08:00–18:00 at **20% below the competitor's usual prices**; usual prices return the next day.

The outage, flu, bulk order, and price war are not preannounced. Snow can be announced through `forecast` events.

Sources: [level definitions](https://spread.bahnexpress.com/kit/HANDBOOK.md#levels), [shortage rules](https://spread.bahnexpress.com/kit/RULES.md#practice-and-the-final).

## 11. API reference and examples

Base URL: `https://spread.bahnexpress.com`. In the table, `W` means `/v1/worlds/{worldId}`. Unless marked public, include the bearer key.

| Method and path | Request / result | Restrictions |
|---|---|---|
| `GET /v1/map` | `{places, roads}` | Public; no rate token. |
| `GET /v1/worlds` | List of world summaries, including `worldId`, `kind`, `level`, `parent`, `season`, `boardRun`, `ended` | Team worlds only. |
| `POST /v1/worlds` | `{ "level": 8, "seed": "study-001" }` → HTTP 201 `{worldId}` | Omit seed for random ordinary practice; use `board:true` without seed for a ranking run. |
| `GET W` | `DeskSnapshot` | Current world state. |
| `POST W/actions` | Array of 1–50 actions → corresponding results | Check each result. |
| `POST W/advance` | One of `{to}`, `{by}`, `{untilEvent:[...],max}`, optionally with actions | Practice only; actions execute before clock movement. |
| `GET W/events?after=123&limit=500` | Events with `seq > 123`, oldest first | `after` defaults to `-1`; `limit` optional. |
| `GET W/drivers` | Driver roster | Can be up to five real seconds stale while the world runs. |
| `POST W/fork` | HTTP 201 `{worldId}` | Ordinary practice only. |
| `GET W/score` | Current `Score` | Early score is not final settlement. |
| `GET W/reveal` | Completed-world `Reveal` | Ordinary practice after end; final after end; never ranking runs. |
| `GET W/stream?after=123` | WebSocket upgrade and JSON events | Use `wss://spread.bahnexpress.com/v1/worlds/{worldId}/stream?...` with authorization. |
| `DELETE W` | HTTP 204 | Ordinary practice deletion; ranking deletion first completes and records the run; forbidden in final. |
| `GET /v1/final` | `{worldId, startsAt, now}` | `startsAt` is an ISO wall-clock timestamp. Can identify an already-ended previous final. |
| `GET /v1/leaderboards/practice?level=8` | `{level,current,previous}` | Public JSON; each round contains teams and per-run outcomes. |
| `GET /v1/leaderboard` | SSE updates about every two seconds | Public; separate from the authenticated world WebSocket. |

### Core object fields

`PriceRequest` contains `id`, `dealerId`, nullable `batchId`, `arrivedAt`, `respondBy`, `legs`, `options`, `referenceFeeCents`, and nullable `note`.

Each leg has:

- `car`: `premium`, `electric`, and nullable `chargePercent`.
- `pickup` and `delivery`: `placeId`, `privateAddress`, `window:{from,to}`, and nullable `hours`.
- Hours: `opens`, `closes`, weekday numbers 0–4, and nullable Saturday opening hours.

An option contains `kind`, `deadline`, and `listPriceCents`. A note is `{text, language}`, with language `fi`, `sv`, or `en`.

`Order` contains `id`, `dealerId`, chosen `option`, `priceCents`, `deadline`, `legs`, `status`, per-leg `deliveredAt`, and `lastPickupAt`. Status is `waiting`, `booked`, `moving`, `delivered`, or `cancelled`.

**Persist the original request.** The order does not repeat its note, typical fee, complete option menu, or batch ID. Change text comes separately in `order_changed.message`.

`Gig` contains `id`, `stops`, `audience`, `schedule`, `status`, `postedAt`, `feeCents`, and nullable `driverId`. Status is `open`, `booked`, `working`, `done`, or `withdrawn`.

`DriverProfile` contains `id`, `homePlaceId`, `rating`, `licence` (`manual` or `automatic_only`), `completedGigs`, and `drops`. A position report contains `driverId`, `placeId`, and `gigId`, without a full movement trace.

`DeskSnapshot` contains `worldId`, `now`, `endsAt`, `openRequests`, `orders`, `gigs`, `driverPositions`, and `score`.

`Score` contains `marginCents`, `revenueCents`, `marginShare`, `wonOrders`, `onTimeShare`, `undeliveredShare`, and `ranked`.

### Action shapes

This compact type sketch describes the public actions. Use the generated SDK types for implementation and the rules for business validation.

```ts
type Service = "same_day" | "next_day" | "within_3_days";
type FeeStep = { at: number; feeCents: number };
type Stop = {
  orderId: string;
  leg: 0 | 1;
  kind: "pickup" | "delivery";
  window?: { from: number; to: number };
  withPlates?: boolean;
  withCharging?: boolean;
};
type Action =
  | { type: "quote"; requestId: string; prices: Partial<Record<Service, number>> }
  | { type: "post"; stops: Stop[];
      audience: { experienced: boolean; manualLicence: boolean };
      schedule: FeeStep[] }
  | { type: "offer"; gigId: string; driverId: string; feeCents: number }
  | { type: "reprice"; gigId: string; schedule: FeeStep[] }
  | { type: "withdraw"; gigId: string }
  | { type: "release"; gigId: string };

type ActionResult =
  | { ok: true; id: string }
  | { ok: false; reason: string };
```

Quote values and fees must be positive integer cents. A post needs at least two stops and both audience booleans. IDs must come from actual API state. Posting a gig returns the ID needed by subsequent offers or repricing; do not assume an arbitrary ID or a dependent action can refer to a yet-unknown result.

`release` the action releases a booked driver; `release` the event announces that a held car is available. They are different operations despite sharing a name.

There are no public actions for forcibly assigning a driver, teleporting/repositioning one, cancelling a won order, or changing competitor prices.

### HTTP examples

Public map and current ranking data:

```bash
curl -fsS "$SPREAD_URL/v1/map"
curl -fsS "$SPREAD_URL/v1/leaderboards/practice?level=8"
```

Create an ordinary practice world:

```bash
curl -fsS -X POST "$SPREAD_URL/v1/worlds" \
  -H "Authorization: Bearer $SPREAD_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"level":8,"seed":"study-001"}'
```

Copy the returned `worldId` into `SPREAD_WORLD_ID`. Read its state:

```bash
curl -fsS "$SPREAD_URL/v1/worlds/$SPREAD_WORLD_ID" \
  -H "Authorization: Bearer $SPREAD_KEY"
```

Advance ordinary practice by one simulated hour:

```bash
curl -fsS -X POST "$SPREAD_URL/v1/worlds/$SPREAD_WORLD_ID/advance" \
  -H "Authorization: Bearer $SPREAD_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"by":60}'
```

Example quote body for `POST W/actions`, using an illustrative ID and price. Replace both with values for a current request and its available options:

```json
[
  {
    "type": "quote",
    "requestId": "REQUEST_ID_FROM_EVENT",
    "prices": { "next_day": 22000 }
  }
]
```

The price above is €220.00 and is an interface example, not a recommended bidding policy.

### SDK method mapping

| Operation | TypeScript | Python |
|---|---|---|
| Client | `new Client()` | `Client()` |
| Create ordinary world | `await client.create(8, {seed:"study-001"})` | `client.create(8, seed="study-001")` |
| Wrap existing world | `client.world(id)` | `client.world(id)` |
| State | `await world.snapshot()` | `world.snapshot()` |
| Events | `await world.events(-1)` | `world.events(after=-1)` |
| Roster | `await world.roster()` | `world.roster()` |
| Map | `await client.map()` | `client.map()` |
| Send actions | `await world.act(actions)` | `world.act(actions)` |
| Event-based practice step | `await world.advance({untilEvent:["price_request"],max:endsAt,actions})` | `world.advance(until=["price_request"],max=ends_at,actions=actions)` |
| Final lookup | `await client.final()` | `client.final()` |
| Stream | `world.stream({after:seq,idleMs:1000})` | `world.stream(after=seq,idle=1.0)` |

SDK `create` and `fork` return a `World` wrapper; raw HTTP returns `{worldId}`. TypeScript stream idle time is milliseconds; Python idle time is seconds. Idle yields `null`/`None`, not an event. Python returns ordinary dictionaries and lists.

Sources: [API reference](https://spread.bahnexpress.com/kit/API.md), [OpenAPI](https://spread.bahnexpress.com/openapi.json), [TypeScript kit](https://spread.bahnexpress.com/kit/spread-typescript.tar.gz), [Python kit](https://spread.bahnexpress.com/kit/spread-0.1.0-py3-none-any.whl).

## 12. Events and state to maintain

Each event includes `seq`, `at` in world minutes, and `kind`. Deduplicate by sequence within a world, and preserve the processed cursor for recovery.

| Event | Main data / interpretation |
|---|---|
| `price_request` | `request`; store its full original contents and quote before `respondBy`. |
| `order_won` | `order`; creates the delivery obligation. |
| `order_lost` | `requestId`; updates quote-outcome learning and releases provisional exposure. |
| `order_changed` | Updated `order` and `message`; reconsider timing, notes, route, and cost. |
| `order_cancelled` | `orderId`, affected gigs and their outcomes; preserve unrelated work. |
| `release` | `orderId`, `leg`; previously held car can be collected. |
| `gig_viewed` | `gigId`, `driverId`; activity signal, not a commitment. |
| `gig_booked` | `gigId`, `driverId`, `feeCents`; driver commitment at the booked fee. |
| `gig_dropped` | `gigId`, `driverId`; gig is open again. |
| `gig_started` | `gigId`, `driverId`; normal driver-release action is now unavailable. |
| `driver_late` | `gigId`, `driverId`, `pickupAt`; driver still comes, free release before departure. |
| `no_show` | `gigId`, `driverId`; gig reopens. |
| `gig_expired` | `gigId`; open gig withdrawn because of a window. |
| `offer_answered` | `gigId`, `driverId`, `answer`: accepted, declined, or expired. |
| `picked_up` | `gigId`, `orderId`, `leg`, `placeId`. |
| `visit_failed` | `gigId`, `orderId`, `leg`, and reason: not ready, closed, licence, or no plates. |
| `delivered` | `gigId`, `orderId`, `leg`, `placeId`; order settles after its last leg. |
| `deadline_close` | `orderId`, `lastPickupAt`; urgent remaining feasibility. |
| `forecast` | Text, start/end minutes, affected place IDs. |
| `ledger` | `line:{at,kind,cents,orderId,gigId,detail}`. |

Ledger kinds are `revenue`, `late_refund`, `undelivered_penalty`, `driver_fee`, `failed_visit`, `release_fee`, `waiting`, and `fine`.

A live `gig_viewed` event does not identify the viewing channel or include the fee the driver saw. Preserve schedule revisions and direct-offer prices to estimate fee exposure. A view associated with a direct offer may involve a different price from the public schedule; ambiguous cases remain uncertain until the completed reveal supplies the viewed `feeCents`.

Recommended application state includes original requests, latest orders, per-leg progress, current gigs and bookings, pending quotes, offer usage by driver/day, note constraints, forecast intervals, dealer outcomes/trust estimates, fee histories, ledger, and event cursor. Keep observed facts separate from estimated availability and cost.

Sources: [event handbook](https://spread.bahnexpress.com/kit/HANDBOOK.md#events), [OpenAPI schemas](https://spread.bahnexpress.com/openapi.json).

## 13. Practice, exports, and ranking rounds

### Ordinary practice

The clock waits until `advance`. The same seed, level, actions, and action times produce the same result. A different policy changes later driver and competitor behavior, so reusing a seed does not make all later events identical across policies.

`advance` may target an absolute minute, a positive number of minutes, or the next named event up to a maximum. Its response includes **all event kinds** since the previous advance, not just the kinds that stopped the clock. Bundled actions execute at the old current time before advancement.

A fork copies the current state and future random draws. Its first advance can include the entire prior history. Handle history carefully rather than applying old orders and ledger entries twice.

There are at most eight unfinished practice worlds, including forks. Ended worlds use no unfinished slot and stay readable. Practice worlds run on the organizer's server; the kit is a client, not a downloadable implementation of the simulator.

The official starter deletes its ordinary practice world on interruption or error. If retaining experiments matters, adapt this error handler intentionally. Deleting an unfinished ranking world has the special consequence described below.

### Reveal and CSV exports

A completed ordinary practice reveal contains:

- Every request, your last quote menu, outcome, and selected option if won.
- For cars you won: actual readiness/release, real site hours, gearbox, plates, tyres, note deadline, pickup and delivery times.
- Gig views, including the fee displayed at each view.

It never discloses competitor prices, private driver costs, or all hidden conditions of un-won cars. An unquoted run supplies request data, not a fully labelled delivery dataset. Keep the event log as well as the reveal.

Export an ended ordinary practice world:

```bash
bun reveal-csv.ts WORLD_ID --out results
```

Or in Python:

```bash
spread-reveal-csv WORLD_ID --out results
```

Generate a request-only dataset in a fresh ordinary world:

```bash
bun reveal-csv.ts --level 8 --seed request-study-001 --out request-data
```

| Export | Contents |
|---|---|
| `requests.csv` | Request features, quote menu, won/lost/unquoted outcome, selected option. |
| `legs.csv` | Per-car route and stated constraints; truth fields where the order was won. |
| `drivers.csv` | Roster information. |
| `views.csv` | Gig, viewer, time, displayed fee. |
| `gigs.csv` | Stops, requirements, schedules, status, driver and fee. |
| `events.csv` | Non-`price_request` events, including outcomes and ledger. |
| `places.csv` | Place IDs, names, coordinates. |
| `roads.csv` | Ordered-pair distance and driving time. |

Nested fields become dotted column names; lists become JSON cells. Use this simulator map for feasibility; external routing services are not the game's authority.

### Ranking rounds

Use `client.create(8,{board:true})` or the starter's `--board`. Each round has five secret weeks per level, played once each, sequentially. Another run at that level cannot begin until the current one ends or is deleted. After the fifth, wait for the next round. Runs count toward the round in which they were created.

Forking and revealing ranking runs are forbidden, including reveal after completion. Deleting an unfinished ranking run lets it run to the end without further decisions, then records the final result. It does not erase the attempt or avoid outstanding liabilities.

To be ranked for a round, **each of the five weeks** must independently meet the delivery requirement and have at least one won order. The five-week margin is summed, and the displayed combined delivery share is order-weighted. A pooled rate over 90% does not rescue one failed week. Ranking rounds inform development but do not contribute points to the final.

Sources: [practice API](https://spread.bahnexpress.com/kit/API.md#practice-worlds), [reveal](https://spread.bahnexpress.com/kit/API.md#the-reveal), [ranking mechanics](https://spread.bahnexpress.com/kit/API.md#practice-rankings).

## 14. Live final, limits, and recovery

### Live clock

The final runs autonomously for approximately 34 real minutes. Its speed changes:

| Simulated period | Real seconds per simulated hour |
|---|---:|
| Main weekdays, 06:00–22:00 | 18 |
| Saturday, 06:00–22:00 | 6 |
| Final Monday, 06:00–22:00 | 9 |
| Nights, Sunday, and final Monday 22:00 through Wednesday 22:00 | 3 |

During main-week weekday daytime hours, a quote's 15 simulated minutes give **4.5 real seconds**. Use server time and event timestamps rather than one constant acceleration factor.

`advance`, `fork`, and deletion are forbidden in the final. Reveal becomes available only after it ends. Use `GET /v1/final` to find your world; a 404 before scheduling is normal. That endpoint may return the last, already-ended final, so check the world's state. The starter's `--final` handles waiting.

Use the authenticated WebSocket for events. Reconnect with `after=<last processed seq>` to replay gaps. A stream without `after` starts with new events only. More than 4 MB of lag closes it with code 1013. Maximum: 16 open streams per team.

The SDK reconnects, but application state still needs reconstruction from history and snapshot. Handle overlap between them without duplicating work. Stream idle heartbeats permit time and termination checks; stop after server state confirms the world's end, not merely after 34 minutes on a local stopwatch.

If your process stops, the world continues: published schedules still run, booked drivers continue, and unanswered requests go to the competitor. If the organizer's server restarts, the explanatory page says the simulation clock pauses and the final finishes correspondingly later.

### Rate limits

| Bucket | Refill | Burst capacity | Charge |
|---|---|---|---|
| Practice | Share of 10,000 requests/sec across recently active teams, capped at 1,000/team; 20/team during a final or test final | One second of tokens | One per request, even if it batches 50 actions. |
| Final actions | 20 tokens/sec | 50 | One per action, even inside a batch. |
| Final reads | 50 tokens/sec | 50 | One per relevant GET or stream opening. |

Practice traffic shares the team bucket across all its processes. Approximate normal rates are 170/sec with 60 practicing teams and 50/sec with 200. A practice request may wait up to one second for tokens; final requests are rejected immediately when over budget. Over-limit responses are 429 and consume no tokens. Public map, leaderboards, kit, and OpenAPI are free of these token charges.

A batch contains at most 50 actions. SDKs split larger lists, so a larger call is not one atomic transaction. In final mode a 50-action batch needs 50 tokens, not one.

Headers: `RateLimit-Policy`, `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`. **`RateLimit-Reset` is milliseconds until one more token**, not a full bucket refill. **`Retry-After` is also milliseconds.** The SDK handles bucket waits and 429 retries.

### Errors and safe recovery

| Response | Meaning |
|---|---|
| 400 | Invalid shape, values, or query; `field` can identify the offending property. |
| 401 | Missing or invalid key. |
| 403 | Inaccessible world or forbidden operation. |
| 404 | Unknown world or no scheduled final. |
| 409 | State conflict or limit: active worlds, streams, early reveal, unfinished/exhausted ranking runs. |
| 429 | Rate limit. |
| 502 / 503 / 504 without JSON | Server/proxy restart; handled by the SDK as `Unreachable`. |
| HTTP 200 with `ok:false` | Individual action failed; inspect every batch result. |

The public contract exposes no idempotency key. After an uncertain mutating response, reconcile state/events before retrying: the action may already have committed. Make event processing repeat-safe and persist the processed sequence. Do not equate SDK reconnection with recovery of your application state.

The TypeScript client uses a 20-second wait for response headers, not a comprehensive full-body deadline. The final starter waits five seconds after `Unreachable`, rebuilds state, and continues. SDKs retry 429 automatically; do not assume all network failures or application bugs are automatically repaired.

Use a supported server runtime for the authorized world WebSocket; an ordinary browser WebSocket constructor cannot set its bearer header. The organizers recommend a stable machine near Stockholm, where their server runs. A dashboard can be separate from the decision process.

Sources: [API final and limits](https://spread.bahnexpress.com/kit/API.md#the-final), [runtime explanation](https://spread.bahnexpress.com/how#final), [TypeScript SDK](https://spread.bahnexpress.com/kit/spread-typescript.tar.gz), [hosting guidance](https://spread.bahnexpress.com/judging).

## 15. Schedule, submission, and judging

**All times below are Helsinki time, EET (UTC+02:00).** The report was prepared on Saturday 14 November. Friday's preliminary deadline is already in the past. The organizer says Junction's current event schedule takes precedence if it changes.

| Date | Time | Event |
|---|---|---|
| Friday 13 November | 19:30 | Hacking and first ranking round opened. |
| Friday 13 November | 23:59 | Preliminary Junction submission deadline. |
| Saturday 14 November | 00:00, 09:00, 12:00, 15:00, 18:00, 21:00 | Published ranking-round schedule. |
| Saturday 14 November | 12:00 | Final submissions open on Junction. |
| Saturday 14 November | 13:45 | First test final set; team registration pauses. |
| Saturday 14 November | 14:00 | First live-clock test, approximately 19 minutes. |
| Saturday 14 November | 19:45 | Second test final set; team registration pauses. |
| Saturday 14 November | 20:00 | Second live-clock test, approximately 19 minutes. |
| Sunday 15 November | 00:00, 03:00, 06:00 | Last ranking rounds. |
| Sunday 15 November | **07:30** | Final is set; team registration closes. |
| Sunday 15 November | **08:00** | Final begins; expected finish around 08:35, subject to interruptions. |
| Sunday 15 November | **10:00** | Final submission deadline. |
| Sunday 15 November | 10:30–14:00 | Review period; challenge jury meets the qualifying teams. |
| Sunday 15 November | 14:00 / 15:10 | Final ceremony / scheduled challenge-winner announcement. |

Registration pauses during each test final until that test ends. Test finals start from a level 8 world's Thursday 00:00 and use the same live interface as the final. They are the opportunity to validate timing, restart recovery, and real-time throughput.

Submit on the Junction platform:

1. The submission hash from your team page.
2. A link to the team's code.
3. A presentation of roughly 5–10 slides or a short video.

For every major idea, explain what it does, why the team tried it, and its measured effect on margin and delivery performance. Compare with and without the idea on the same weeks. Show final and ranking results, failed ideas, and known limits. The jury values a small number of understandable decisions backed by evidence.

The supplied challenge listing advertised prizes of **€2,000 / €1,000 / €500** for first, second, and third place respectively; confirm any event changes with the booth.

Source: [official schedule and judging](https://spread.bahnexpress.com/judging); prize amounts from the supplied challenge-listing screenshot.

## 16. Benchmarks, remaining inputs, and sources

### Organizer benchmark

The homepage reports these results on the same 20 level 8 weeks:

| System | Reported average weekly margin | Eligible weeks |
|---|---:|---:|
| House starter | −€67,177 | 0 / 20 |
| House best | €12,551 | 20 / 20 |
| Privileged oracle | €16,578 | 7 / 20 |

The oracle sees hidden competitor prices and driver costs, but still takes too much work and misses the delivery requirement. Its reported margin is not a proven upper bound. House best was built with the participant SDK and public API. These are organizer-reported results, not measurements of the proposal. Live standings change; use the public ranking endpoint for current comparisons.

Source: [homepage benchmark](https://spread.bahnexpress.com/).

### Inputs needed to start implementation and evaluation

- A team registered before the relevant cutoff and its key configured as `SPREAD_KEY` in the execution runtime.
- The current official SDK and this handoff.
- Any existing team repository or run exports, if work has already started.
- A stable machine for the final. Team size and model-provider preferences affect scope but are not required to begin ordinary implementation.

No private simulator source or hidden competitor prices are required. Practice access is needed to measure and tune performance. This guide was assembled using public documents and SDK inspection; no authenticated practice run, ranking attempt, or final action was performed to produce it.

### Source index

- [Official challenge and benchmarks](https://spread.bahnexpress.com/)
- [Human explanation](https://spread.bahnexpress.com/how)
- [Handbook](https://spread.bahnexpress.com/kit/HANDBOOK.md)
- [Authoritative rules, dated 13 November 2026](https://spread.bahnexpress.com/kit/RULES.md)
- [HTTP API reference](https://spread.bahnexpress.com/kit/API.md)
- [OpenAPI contract](https://spread.bahnexpress.com/openapi.json)
- [TypeScript SDK](https://spread.bahnexpress.com/kit/spread-typescript.tar.gz)
- [Python SDK](https://spread.bahnexpress.com/kit/spread-0.1.0-py3-none-any.whl)
- [Submission and judging](https://spread.bahnexpress.com/judging)
- [Current level 8 ranking data](https://spread.bahnexpress.com/v1/leaderboards/practice?level=8)

