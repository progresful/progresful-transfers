// Package api holds the wire types and HTTP client for the Spread API (version 0).
// Times are world minutes from Monday 00:00; money is integer game-euro cents.
package api

import "encoding/json"

// Delivery option kinds.
const (
	SameDay     = "same_day"
	NextDay     = "next_day"
	Within3Days = "within_3_days"
)

// Event kinds.
const (
	KindPriceRequest   = "price_request"
	KindOrderWon       = "order_won"
	KindOrderLost      = "order_lost"
	KindOrderChanged   = "order_changed"
	KindOrderCancelled = "order_cancelled"
	KindRelease        = "release"
	KindGigViewed      = "gig_viewed"
	KindGigBooked      = "gig_booked"
	KindGigDropped     = "gig_dropped"
	KindGigStarted     = "gig_started"
	KindDriverLate     = "driver_late"
	KindNoShow         = "no_show"
	KindGigExpired     = "gig_expired"
	KindOfferAnswered  = "offer_answered"
	KindPickedUp       = "picked_up"
	KindVisitFailed    = "visit_failed"
	KindDelivered      = "delivered"
	KindDeadlineClose  = "deadline_close"
	KindForecast       = "forecast"
	KindLedger         = "ledger"
)

// Action types.
const (
	ActQuote    = "quote"
	ActPost     = "post"
	ActOffer    = "offer"
	ActReprice  = "reprice"
	ActWithdraw = "withdraw"
	ActRelease  = "release"
)

type TimeWindow struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type StopRef struct {
	OrderID      string      `json:"orderId"`
	Leg          int         `json:"leg"`
	Kind         string      `json:"kind"` // "pickup" | "delivery"
	Window       *TimeWindow `json:"window,omitempty"`
	WithPlates   bool        `json:"withPlates,omitempty"`
	WithCharging bool        `json:"withCharging,omitempty"`
}

type Audience struct {
	Experienced   bool `json:"experienced"`
	ManualLicence bool `json:"manualLicence"`
}

type FeeStep struct {
	At       int `json:"at"`
	FeeCents int `json:"feeCents"`
}

// Action is the union of all dispatcher actions; only the fields of its Type are set.
type Action struct {
	Type      string         `json:"type"`
	RequestID string         `json:"requestId,omitempty"`
	Prices    map[string]int `json:"prices,omitempty"`
	Stops     []StopRef      `json:"stops,omitempty"`
	Audience  *Audience      `json:"audience,omitempty"`
	Schedule  []FeeStep      `json:"schedule,omitempty"`
	GigID     string         `json:"gigId,omitempty"`
	DriverID  string         `json:"driverId,omitempty"`
	FeeCents  int            `json:"feeCents,omitempty"`
}

type ActionResult struct {
	OK     bool   `json:"ok"`
	ID     string `json:"id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type Car struct {
	Premium       bool `json:"premium"`
	Electric      bool `json:"electric"`
	ChargePercent *int `json:"chargePercent"`
}

type Saturday struct {
	Opens  int `json:"opens"`
	Closes int `json:"closes"`
}

// OpeningHours are minutes within a day (not world minutes).
type OpeningHours struct {
	Opens    int       `json:"opens"`
	Closes   int       `json:"closes"`
	Weekdays []int     `json:"weekdays"`
	Saturday *Saturday `json:"saturday"`
}

type StopPlace struct {
	PlaceID        string        `json:"placeId"`
	PrivateAddress bool          `json:"privateAddress"`
	Window         TimeWindow    `json:"window"`
	Hours          *OpeningHours `json:"hours"`
}

type Leg struct {
	Car      Car       `json:"car"`
	Pickup   StopPlace `json:"pickup"`
	Delivery StopPlace `json:"delivery"`
}

type DeliveryOption struct {
	Kind           string `json:"kind"`
	Deadline       int    `json:"deadline"`
	ListPriceCents int    `json:"listPriceCents"`
}

type Note struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}

type PriceRequest struct {
	ID                string           `json:"id"`
	DealerID          string           `json:"dealerId"`
	BatchID           *string          `json:"batchId"`
	ArrivedAt         int              `json:"arrivedAt"`
	RespondBy         int              `json:"respondBy"`
	Legs              []Leg            `json:"legs"`
	Options           []DeliveryOption `json:"options"`
	ReferenceFeeCents int              `json:"referenceFeeCents"`
	Note              *Note            `json:"note"`
}

type Order struct {
	ID           string `json:"id"`
	DealerID     string `json:"dealerId"`
	Option       string `json:"option"`
	PriceCents   int    `json:"priceCents"`
	Deadline     int    `json:"deadline"`
	Legs         []Leg  `json:"legs"`
	Status       string `json:"status"` // waiting | booked | moving | delivered | cancelled
	DeliveredAt  []*int `json:"deliveredAt"`
	LastPickupAt int    `json:"lastPickupAt"`
}

type CancelledGig struct {
	GigID   string `json:"gigId"`
	Outcome string `json:"outcome"` // withdrawn | released | shortened | continues
}

type LedgerLine struct {
	At      int     `json:"at"`
	Kind    string  `json:"kind"`
	Cents   int     `json:"cents"`
	OrderID *string `json:"orderId"`
	GigID   *string `json:"gigId"`
	Detail  string  `json:"detail"`
}

// Event is the union of all desk events. Fields not used by a kind stay zero.
// Note that `from`/`to` belong to forecast events and `leg` to several kinds.
type Event struct {
	Seq  int    `json:"seq"`
	At   int    `json:"at"`
	Kind string `json:"kind"`

	Request      *PriceRequest  `json:"request,omitempty"`
	Order        *Order         `json:"order,omitempty"`
	Message      *Note          `json:"message,omitempty"`
	RequestID    string         `json:"requestId,omitempty"`
	OrderID      string         `json:"orderId,omitempty"`
	Gigs         []CancelledGig `json:"gigs,omitempty"`
	Leg          int            `json:"leg,omitempty"`
	GigID        string         `json:"gigId,omitempty"`
	DriverID     string         `json:"driverId,omitempty"`
	FeeCents     int            `json:"feeCents,omitempty"`
	PickupAt     int            `json:"pickupAt,omitempty"`
	Answer       string         `json:"answer,omitempty"`
	PlaceID      string         `json:"placeId,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	LastPickupAt int            `json:"lastPickupAt,omitempty"`
	Text         string         `json:"text,omitempty"`
	From         int            `json:"from,omitempty"`
	To           int            `json:"to,omitempty"`
	PlaceIDs     []string       `json:"placeIds,omitempty"`
	Line         *LedgerLine    `json:"line,omitempty"`

	// Raw is the event exactly as the server sent it, kept for the run log.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the typed fields and keeps the raw bytes.
func (e *Event) UnmarshalJSON(data []byte) error {
	type plain Event
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*e = Event(p)
	e.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Gig struct {
	ID       string    `json:"id"`
	Stops    []StopRef `json:"stops"`
	Audience Audience  `json:"audience"`
	Schedule []FeeStep `json:"schedule"`
	Status   string    `json:"status"` // open | booked | working | done | withdrawn
	PostedAt int       `json:"postedAt"`
	FeeCents int       `json:"feeCents"`
	DriverID *string   `json:"driverId"`
}

type DriverPosition struct {
	DriverID string `json:"driverId"`
	PlaceID  string `json:"placeId"`
	GigID    string `json:"gigId"`
}

type Score struct {
	MarginCents      int     `json:"marginCents"`
	RevenueCents     int     `json:"revenueCents"`
	MarginShare      float64 `json:"marginShare"`
	WonOrders        int     `json:"wonOrders"`
	OnTimeShare      float64 `json:"onTimeShare"`
	UndeliveredShare float64 `json:"undeliveredShare"`
	Ranked           bool    `json:"ranked"`
}

type Snapshot struct {
	WorldID         string           `json:"worldId"`
	Now             int              `json:"now"`
	EndsAt          int              `json:"endsAt"`
	OpenRequests    []PriceRequest   `json:"openRequests"`
	Orders          []Order          `json:"orders"`
	Gigs            []Gig            `json:"gigs"`
	DriverPositions []DriverPosition `json:"driverPositions"`
	Score           Score            `json:"score"`
}

type DriverProfile struct {
	ID            string  `json:"id"`
	HomePlaceID   string  `json:"homePlaceId"`
	Rating        float64 `json:"rating"`
	Licence       string  `json:"licence"` // manual | automatic_only
	CompletedGigs int     `json:"completedGigs"`
	Drops         int     `json:"drops"`
}

type WorldSummary struct {
	WorldID  string  `json:"worldId"`
	Kind     string  `json:"kind"`
	Level    int     `json:"level"`
	Parent   *string `json:"parent"`
	Season   *int    `json:"season"`
	BoardRun *int    `json:"boardRun"`
	Ended    bool    `json:"ended"`
}

type Advanced struct {
	Results []ActionResult `json:"results"`
	Now     int            `json:"now"`
	Events  []Event        `json:"events"`
}

type Final struct {
	WorldID  string `json:"worldId"`
	StartsAt string `json:"startsAt"`
	Now      int    `json:"now"`
}

type Place struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type Road struct {
	From    string  `json:"from"`
	To      string  `json:"to"`
	Km      float64 `json:"km"`
	Minutes int     `json:"minutes"`
}

type WorldMap struct {
	Places []Place `json:"places"`
	Roads  []Road  `json:"roads"`
}

// Reveal types (completed ordinary practice worlds and the ended final).

type RevealedRequest struct {
	Request PriceRequest   `json:"request"`
	Quote   map[string]int `json:"quote"`
	Outcome string         `json:"outcome"` // won | lost | unquoted
	Option  *string        `json:"option"`
}

type LegTruth struct {
	ReadyAt       int           `json:"readyAt"`
	ReleasedAt    *int          `json:"releasedAt"`
	PickupHours   *OpeningHours `json:"pickupHours"`
	DeliveryHours *OpeningHours `json:"deliveryHours"`
	ManualGearbox bool          `json:"manualGearbox"`
	NeedsPlates   bool          `json:"needsPlates"`
	SummerTyres   bool          `json:"summerTyres"`
	NoteDeadline  *int          `json:"noteDeadline"`
	PickedUpAt    *int          `json:"pickedUpAt"`
	DeliveredAt   *int          `json:"deliveredAt"`
}

type RevealedOrder struct {
	OrderID string     `json:"orderId"`
	Legs    []LegTruth `json:"legs"`
}

type View struct {
	GigID    string `json:"gigId"`
	DriverID string `json:"driverId"`
	At       int    `json:"at"`
	FeeCents int    `json:"feeCents"`
}

type Reveal struct {
	Requests []RevealedRequest `json:"requests"`
	Orders   []RevealedOrder   `json:"orders"`
	Views    []View            `json:"views"`
}

// Practice rankings.

type BoardRun struct {
	Run         int     `json:"run"`
	MarginCents int     `json:"marginCents"`
	OnTimeShare float64 `json:"onTimeShare"`
	WonOrders   int     `json:"wonOrders"`
	Ranked      bool    `json:"ranked"`
	DeletedAt   *int    `json:"deletedAt"`
}

type BoardTeam struct {
	Rank        *int       `json:"rank"`
	Team        string     `json:"team"`
	Name        string     `json:"name"`
	Played      int        `json:"played"`
	Ranked      bool       `json:"ranked"`
	MarginCents *int       `json:"marginCents"`
	OnTimeShare *float64   `json:"onTimeShare"`
	Runs        []BoardRun `json:"runs"`
}

type BoardSeason struct {
	Season    int         `json:"season"`
	Name      string      `json:"name"`
	StartedAt string      `json:"startedAt"`
	Teams     []BoardTeam `json:"teams"`
}

type PracticeBoard struct {
	Level    int          `json:"level"`
	Current  *BoardSeason `json:"current"`
	Previous *BoardSeason `json:"previous"`
}
