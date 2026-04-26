package main

import (
	"encoding/json"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// ─── Resource tracker ───────────────────────────────────────────────────────

type Resource struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`     // water/food/medicine/battery/fuel/other
	Qty     float64   `json:"qty"`      // current units
	Unit    string    `json:"unit"`     // L, kg, doses, %, hr
	BurnPer float64   `json:"burnPer"`  // units consumed per hour (0 = static)
	Updated time.Time `json:"updated"`
}

// ─── Map pins ───────────────────────────────────────────────────────────────

type MapPin struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Note  string    `json:"note"`
	Kind  string    `json:"kind"` // safe / danger / supply / shelter / water
	X     float64   `json:"x"`    // -100..100 grid coord
	Y     float64   `json:"y"`
	At    time.Time `json:"at"`
}

// ─── Vitals (own health) ────────────────────────────────────────────────────

type Vitals struct {
	Updated time.Time `json:"updated"`
	HR      int       `json:"hr"`     // beats per min
	SpO2    int       `json:"spo2"`   // oxygen saturation %
	Temp    float64   `json:"temp"`   // Celsius
	Hydrate int       `json:"hydrate"` // 0-100 %
	Notes   string    `json:"notes"`
}

// ─── Safety store ───────────────────────────────────────────────────────────

type safetyStore struct {
	mu       sync.RWMutex
	prefs    fyne.Preferences
	res      []Resource
	pins     []MapPin
	vitals   Vitals
	listener func()
}

var Safety = &safetyStore{}

func (s *safetyStore) Bind(p fyne.Preferences) {
	s.prefs = p
	s.load()
}

func (s *safetyStore) SetListener(f func()) { s.listener = f }
func (s *safetyStore) notify() {
	if s.listener != nil {
		s.listener()
	}
}

func (s *safetyStore) load() {
	if s.prefs == nil {
		return
	}
	if str := s.prefs.String("safety.res"); str != "" {
		_ = json.Unmarshal([]byte(str), &s.res)
	}
	if str := s.prefs.String("safety.pins"); str != "" {
		_ = json.Unmarshal([]byte(str), &s.pins)
	}
	if str := s.prefs.String("safety.vitals"); str != "" {
		_ = json.Unmarshal([]byte(str), &s.vitals)
	}
	// Seed defaults on first run
	if len(s.res) == 0 {
		s.res = []Resource{
			{ID: randomHex(4), Name: "Drinking water", Kind: "water", Qty: 5, Unit: "L", BurnPer: 0.125, Updated: time.Now()},
			{ID: randomHex(4), Name: "Rations", Kind: "food", Qty: 12, Unit: "meals", BurnPer: 0.125, Updated: time.Now()},
			{ID: randomHex(4), Name: "Phone battery", Kind: "battery", Qty: 100, Unit: "%", BurnPer: 4, Updated: time.Now()},
		}
		s.save()
	}
}

func (s *safetyStore) save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.prefs == nil {
		return
	}
	if b, err := json.Marshal(s.res); err == nil {
		s.prefs.SetString("safety.res", string(b))
	}
	if b, err := json.Marshal(s.pins); err == nil {
		s.prefs.SetString("safety.pins", string(b))
	}
	if b, err := json.Marshal(s.vitals); err == nil {
		s.prefs.SetString("safety.vitals", string(b))
	}
}

// Resources

func (s *safetyStore) Resources() []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Resource, len(s.res))
	copy(out, s.res)
	return out
}

func (s *safetyStore) AddResource(r Resource) {
	if r.ID == "" {
		r.ID = randomHex(4)
	}
	r.Updated = time.Now()
	s.mu.Lock()
	s.res = append(s.res, r)
	s.mu.Unlock()
	s.save()
	s.notify()
}

func (s *safetyStore) UpdateResource(id string, qty float64) {
	s.mu.Lock()
	for i := range s.res {
		if s.res[i].ID == id {
			s.res[i].Qty = qty
			s.res[i].Updated = time.Now()
		}
	}
	s.mu.Unlock()
	s.save()
	s.notify()
}

func (s *safetyStore) DeleteResource(id string) {
	s.mu.Lock()
	out := s.res[:0]
	for _, r := range s.res {
		if r.ID != id {
			out = append(out, r)
		}
	}
	s.res = out
	s.mu.Unlock()
	s.save()
	s.notify()
}

// HoursLeft returns estimated hours until depletion (0 if static).
func (r Resource) HoursLeft() float64 {
	if r.BurnPer <= 0 {
		return 0
	}
	return r.Qty / r.BurnPer
}

// Pins

func (s *safetyStore) Pins() []MapPin {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MapPin, len(s.pins))
	copy(out, s.pins)
	return out
}

func (s *safetyStore) AddPin(p MapPin) {
	if p.ID == "" {
		p.ID = randomHex(4)
	}
	p.At = time.Now()
	s.mu.Lock()
	s.pins = append(s.pins, p)
	s.mu.Unlock()
	s.save()
	s.notify()
}

func (s *safetyStore) DeletePin(id string) {
	s.mu.Lock()
	out := s.pins[:0]
	for _, p := range s.pins {
		if p.ID != id {
			out = append(out, p)
		}
	}
	s.pins = out
	s.mu.Unlock()
	s.save()
	s.notify()
}

// Vitals

func (s *safetyStore) GetVitals() Vitals {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vitals
}

func (s *safetyStore) SetVitals(v Vitals) {
	v.Updated = time.Now()
	s.mu.Lock()
	s.vitals = v
	s.mu.Unlock()
	s.save()
	s.notify()
}

// ─── Static reference content ──────────────────────────────────────────────

type FirstAidEntry struct {
	Title string
	Tag   string // BLEEDING, CPR, BURN, SHOCK, etc.
	Steps []string
}

var FirstAidLibrary = []FirstAidEntry{
	{
		Title: "Severe bleeding",
		Tag:   "BLEEDING",
		Steps: []string{
			"Apply direct pressure with cleanest cloth available.",
			"Elevate the wound above the heart if possible.",
			"Pack deep wounds — do NOT remove embedded objects.",
			"If pressure fails on a limb, apply a tourniquet 5 cm above wound, mark TIME on it.",
			"Treat for shock: keep warm, lay flat, legs raised slightly.",
		},
	},
	{
		Title: "CPR (adult, no breathing)",
		Tag:   "CPR",
		Steps: []string{
			"Check responsiveness, shout for help.",
			"30 chest compressions: center of chest, 5–6 cm deep, 100–120/min.",
			"2 rescue breaths if trained — otherwise compressions only.",
			"Continue 30:2 cycles until help arrives or person breathes.",
			"Do NOT stop unless physically unable.",
		},
	},
	{
		Title: "Burns (thermal)",
		Tag:   "BURN",
		Steps: []string{
			"Cool with running water (not ice) for at least 20 minutes.",
			"Remove jewelry/clothing near the burn before swelling.",
			"Cover loosely with sterile non-stick dressing or cling film.",
			"Do NOT pop blisters or apply butter/oils.",
			"Seek medical help if larger than the palm or on face/hands/joints.",
		},
	},
	{
		Title: "Shock (circulatory)",
		Tag:   "SHOCK",
		Steps: []string{
			"Lay flat; elevate legs ~30 cm unless head/spine injury suspected.",
			"Keep warm with blanket / clothing.",
			"Loosen tight clothing.",
			"Do NOT give food or water.",
			"Reassure, monitor breathing & pulse until help arrives.",
		},
	},
	{
		Title: "Hypothermia",
		Tag:   "COLD",
		Steps: []string{
			"Move to dry, sheltered spot. Remove wet clothes.",
			"Warm core first: blankets, dry layers, body-to-body if available.",
			"Sugary warm drinks ONLY if fully conscious.",
			"Do NOT rub limbs or apply direct intense heat.",
			"Severe (confused, drowsy): emergency evac required.",
		},
	},
	{
		Title: "Heat stroke",
		Tag:   "HEAT",
		Steps: []string{
			"Move to coolest available shade.",
			"Cool aggressively: wet skin + fanning, ice packs to neck / armpits / groin.",
			"Sip cool water if conscious & able to swallow.",
			"Do NOT give meds (paracetamol / aspirin) — ineffective in heat stroke.",
			"Body temp >40 °C with confusion = medical emergency.",
		},
	},
	{
		Title: "Choking (adult, conscious)",
		Tag:   "CHOKE",
		Steps: []string{
			"Encourage forceful coughing.",
			"5 sharp back-blows between shoulder blades.",
			"5 abdominal thrusts (Heimlich) above navel.",
			"Alternate back-blows / thrusts until cleared or unconscious.",
			"If unconscious: begin CPR.",
		},
	},
	{
		Title: "Fracture (suspected)",
		Tag:   "BREAK",
		Steps: []string{
			"Immobilise the limb in the position found.",
			"Pad and splint with rigid materials, secure above & below break.",
			"Check circulation past the splint (warmth / pulse / colour).",
			"Apply cold pack briefly to reduce swelling.",
			"Do NOT attempt to realign unless arterial supply is lost.",
		},
	},
}

// Survival checklists (3-2-1 tactical hierarchy)

type Checklist struct {
	Title string
	Tag   string
	Items []string
}

var SurvivalChecklists = []Checklist{
	{
		Title: "Bug-out bag (24 hr)",
		Tag:   "BAG",
		Items: []string{
			"3 L water + filter / purification tabs",
			"High-cal food (energy bars, jerky)",
			"Compact first-aid kit + personal meds (3 days)",
			"Multi-tool, sharp knife, paracord 10 m",
			"Headlamp + spare batteries / hand-crank torch",
			"Emergency blanket, poncho, fire starter (×2)",
			"Phone power bank, paper map, compass",
			"Cash, ID copy, contact list on paper",
			"Whistle, signal mirror, dust mask N95",
			"Spare socks, gloves, beanie",
		},
	},
	{
		Title: "Shelter selection",
		Tag:   "SHELTER",
		Items: []string{
			"Above flood line, off ridges (lightning).",
			"Out of wind path, near firewood + water.",
			"NOT in dry creek beds (flash flood).",
			"Two exit routes, line of sight to landmarks.",
			"Mark location for rescuers (X / SOS / fire).",
		},
	},
	{
		Title: "Water purification (no filter)",
		Tag:   "WATER",
		Items: []string{
			"Settle sediment 1 hr in clean container.",
			"Boil at full rolling boil ≥ 1 min (≥ 3 min above 2000 m).",
			"OR 8 drops 5% bleach per litre, wait 30 min.",
			"OR solar (SODIS): clear PET bottle 6 hr in direct sun.",
			"OR iodine 5 drops/L, wait 30 min (avoid for pregnancy/thyroid).",
		},
	},
	{
		Title: "Conflict / shelling",
		Tag:   "CONFLICT",
		Items: []string{
			"Stay LOW — basement, interior room, away from windows.",
			"Two solid walls between you and outside (\"two walls rule\").",
			"Fill bathtub & containers with water immediately.",
			"Charge devices, prepare battery radio for civil defence freqs.",
			"Mark FIRST AID supplies; assign a triage corner.",
			"Plan evac route: avoid bridges, fuel stations, military installations.",
		},
	},
	{
		Title: "Earthquake immediate",
		Tag:   "QUAKE",
		Items: []string{
			"DROP, COVER, HOLD ON under sturdy table.",
			"Stay clear of windows, mirrors, heavy furniture.",
			"If outdoors: open ground, away from buildings & power lines.",
			"After shaking: check for gas, fire, injuries.",
			"Expect aftershocks for 24–72 hr — keep shoes on.",
		},
	},
}

