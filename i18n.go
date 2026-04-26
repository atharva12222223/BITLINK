package main

import "sync"

type Lang string

const (
	LangEN Lang = "en"
	LangHI Lang = "hi"
)

var (
	currentLang   Lang = LangEN
	currentLangMu sync.RWMutex
)

func CurrentLang() Lang {
	currentLangMu.RLock()
	defer currentLangMu.RUnlock()
	return currentLang
}

func SetLang(l Lang) {
	currentLangMu.Lock()
	currentLang = l
	currentLangMu.Unlock()
	if prefs != nil {
		prefs.SetString("app.lang", string(l))
	}
}

func LoadLang() {
	if prefs == nil {
		return
	}
	if s := prefs.String("app.lang"); s != "" {
		currentLangMu.Lock()
		currentLang = Lang(s)
		currentLangMu.Unlock()
	}
}

func LangDisplayName(l Lang) string {
	switch l {
	case LangHI:
		return "हिन्दी (Hindi)"
	default:
		return "English"
	}
}

func AvailableLangs() []Lang { return []Lang{LangEN, LangHI} }

// T translates a key to the current language.
func T(key string) string {
	lang := CurrentLang()
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := translations[LangEN][key]; ok {
		return v
	}
	return key
}

var translations = map[Lang]map[string]string{
	LangEN: {
		// Nav
		"nav.home": "Home", "nav.chat": "Chat", "nav.groups": "Groups",
		"nav.map": "Map", "nav.safety": "Safety", "nav.contacts": "Contacts",
		"nav.mesh": "Mesh", "nav.settings": "Settings", "nav.navigation": "Navigation",
		// Headers
		"hdr.private_chats": "Private Chats", "hdr.1to1": "1-TO-1 LINKS",
		"hdr.groups": "Group Channels", "hdr.mesh_broadcasts": "MESH BROADCASTS",
		"hdr.map": "Tactical Map", "hdr.map_sub": "OFFLINE PIN BOARD",
		"nav.compass": "Compass", "hdr.compass_sub": "BEARING TOOL",
		"compass.set_bearing": "Set", "compass.controls": "Controls",
		"compass.bearing": "Bearing (degrees)",
		"safety.search": "Search guides...",
		"hdr.safety": "Safety & Survival", "hdr.safety_sub": "OFFLINE FIELD KIT",
		"hdr.contacts": "Contacts", "hdr.contacts_sub": "SAVED PEERS",
		"hdr.mesh": "Mesh Topology", "hdr.mesh_sub": "LIVE GRAPH",
		"hdr.settings": "Settings", "hdr.settings_sub": "PROFILE & MESH",
		// Settings
		"set.identity": "Identity", "set.display_name": "Display name",
		"set.avatar_color": "Avatar color", "set.save_profile": "Save Profile",
		"set.saved": "Saved", "set.profile_updated": "Profile updated.",
		"set.mesh": "Mesh", "set.ttl": "Default TTL (hops)",
		"set.advertise": "Advertise this node over BLE",
		"set.data": "Data", "set.clear_chat": "Clear ALL chat history",
		"set.hard_reset": "HARD RESET — wipe everything",
		"set.about": "About", "set.language": "Language",
		"set.lang_restart": "Language changed. Restart the app to apply.",
		"set.local_data": "All data lives locally on this device. No server, no telemetry.",
		"set.nuclear": "Nuclear option — clears username, contacts, chats, everything.",
		// Chat
		"chat.select": "Select a contact",
		"chat.select_desc": "Pick a peer on the left to open a 1-to-1 secure mesh chat.",
		"chat.type": "Type a message...", "chat.send": "Send",
		"chat.search": "Search messages...", "chat.export": "Export",
		"chat.no_messages": "No messages yet", "chat.start_desc": "Say hi to start the conversation!",
		// Contacts
		"con.no_peers": "No peers in range",
		"con.discover": "Tap BEGIN DISCOVERY on the Home tab to start scanning the mesh.",
		"con.saved": "Saved", "con.discovered": "Discovered",
		"con.save": "Save", "con.nickname": "Nickname",
		"con.save_contact": "Save Contact", "con.cancel": "Cancel",
		// Map
		"map.drop_pin": "Drop Pin (center)", "map.clear_pins": "Clear all pins",
		"map.safe": "Safe", "map.danger": "Danger", "map.supply": "Supply",
		"map.shelter": "Shelter", "map.other": "Other",
		"map.help": "Tap anywhere on the grid to drop a marker.",
		// SOS
		"sos.broadcast": "BROADCAST SOS", "sos.incoming": "INCOMING SOS",
		"sos.from": "from", "sos.dismiss": "Acknowledge",
		"sos.message": "Message", "sos.broadcast_title": "Broadcast SOS",
		// Status
		"status.online": "ONLINE", "status.offline": "OFFLINE",
		"status.linked": "LINKED", "status.in_range": "IN RANGE",
		"status.discovered": "DISCOVERED", "status.mesh_operator": "MESH OPERATOR",
		// General
		"gen.peers": "peers", "gen.peer": "peer",
		// Welcome
		"welcome.title": "Welcome to BitLink",
		"welcome.tagline": "Your offline mesh starts here.",
		"welcome.local": "Stored locally — no account, no server.",
		"welcome.begin": "Begin", "welcome.skip": "Skip",
		// Groups
		"grp.create": "Create Group", "grp.name": "Group name",
		"grp.no_groups": "No groups yet",
		// Safety
		"safety.first_aid": "First-Aid Quick Reference",
		"safety.checklists": "Survival Checklists",
		// Home
		"home.hero_eyebrow": "MESH DISCOVERY",
		"home.scan_title": "Scan for Devices",
		"home.scan_sub1": "Initiate a wide-spectrum mesh broadcast to identify nodes within range.",
		"home.scan_sub2": "Encrypted discovery handshake is automated.",
		"home.ready": "READY • IDLE",
		"home.begin_discovery": "BEGIN DISCOVERY",
		"home.scanning": "▰▰▰  SCANNING  ▰▰▰",
		"home.found_nodes": "FOUND %d NODES IN RANGE",
		"home.stat_peers": "PEERS", "home.stat_linked": "LINKED", "home.stat_unread": "UNREAD",
		"home.quick_actions": "Quick Actions",
		"home.qa_chat_desc": "OPEN PRIVATE LINKS",
		"home.qa_groups_desc": "WHATSAPP-STYLE CHANNELS",
		"home.qa_compass_desc": "BEARING TOOL",
		"home.qa_safety_desc": "FIRST-AID & SURVIVAL",
	},
	LangHI: {
		"nav.home": "होम", "nav.chat": "चैट", "nav.groups": "ग्रुप्स",
		"nav.map": "मैप", "nav.safety": "सुरक्षा", "nav.contacts": "संपर्क",
		"nav.mesh": "मेश", "nav.settings": "सेटिंग्स", "nav.navigation": "नेविगेशन",
		"hdr.private_chats": "निजी चैट", "hdr.1to1": "1-टू-1 लिंक",
		"hdr.groups": "ग्रुप चैनल", "hdr.mesh_broadcasts": "मेश ब्रॉडकास्ट",
		"hdr.map": "सामरिक मैप", "hdr.map_sub": "ऑफलाइन पिन बोर्ड",
		"nav.compass": "कम्पास", "hdr.compass_sub": "दिशा उपकरण",
		"compass.set_bearing": "सेट", "compass.controls": "नियंत्रण",
		"compass.bearing": "दिशा (डिग्री)",
		"safety.search": "गाइड खोजें...",
		"hdr.safety": "सुरक्षा और उत्तरजीविता", "hdr.safety_sub": "ऑफलाइन फील्ड किट",
		"hdr.contacts": "संपर्क", "hdr.contacts_sub": "सहेजे गए पीयर",
		"hdr.mesh": "मेश टोपोलॉजी", "hdr.mesh_sub": "लाइव ग्राफ",
		"hdr.settings": "सेटिंग्स", "hdr.settings_sub": "प्रोफाइल और मेश",
		"set.identity": "पहचान", "set.display_name": "प्रदर्शन नाम",
		"set.avatar_color": "अवतार रंग", "set.save_profile": "प्रोफाइल सहेजें",
		"set.saved": "सहेजा गया", "set.profile_updated": "प्रोफाइल अपडेट हो गई।",
		"set.mesh": "मेश", "set.ttl": "डिफ़ॉल्ट TTL (हॉप्स)",
		"set.advertise": "BLE पर इस नोड का विज्ञापन करें",
		"set.data": "डेटा", "set.clear_chat": "सभी चैट इतिहास साफ़ करें",
		"set.hard_reset": "हार्ड रीसेट — सब कुछ मिटाएं",
		"set.about": "ऐप के बारे में", "set.language": "भाषा",
		"set.lang_restart": "भाषा बदली गई। लागू करने के लिए ऐप रीस्टार्ट करें।",
		"set.local_data": "सारा डेटा इस डिवाइस पर है। कोई सर्वर नहीं।",
		"set.nuclear": "सब कुछ मिटा दें — यूज़रनेम, संपर्क, चैट, सब।",
		"chat.select": "संपर्क चुनें",
		"chat.select_desc": "1-टू-1 मेश चैट शुरू करने के लिए बाईं ओर एक पीयर चुनें।",
		"chat.type": "संदेश लिखें...", "chat.send": "भेजें",
		"chat.search": "संदेश खोजें...", "chat.export": "निर्यात",
		"chat.no_messages": "अभी कोई संदेश नहीं", "chat.start_desc": "बातचीत शुरू करने के लिए हैलो कहें!",
		"con.no_peers": "रेंज में कोई पीयर नहीं",
		"con.discover": "मेश स्कैन शुरू करने के लिए होम टैब पर डिस्कवरी शुरू करें पर टैप करें।",
		"con.saved": "सहेजे गए", "con.discovered": "खोजे गए",
		"con.save": "सहेजें", "con.nickname": "उपनाम",
		"con.save_contact": "संपर्क सहेजें", "con.cancel": "रद्द करें",
		"map.drop_pin": "पिन डालें (केंद्र)", "map.clear_pins": "सभी पिन हटाएं",
		"map.safe": "सुरक्षित", "map.danger": "खतरा", "map.supply": "आपूर्ति",
		"map.shelter": "आश्रय", "map.other": "अन्य",
		"map.help": "मार्कर रखने के लिए ग्रिड पर कहीं भी टैप करें।",
		"sos.broadcast": "SOS प्रसारित करें", "sos.incoming": "आने वाला SOS",
		"sos.from": "से", "sos.dismiss": "स्वीकार करें",
		"sos.message": "संदेश", "sos.broadcast_title": "SOS प्रसारित करें",
		"status.online": "ऑनलाइन", "status.offline": "ऑफलाइन",
		"status.linked": "जुड़ा हुआ", "status.in_range": "रेंज में",
		"status.discovered": "खोजा गया", "status.mesh_operator": "मेश ऑपरेटर",
		"welcome.title": "BitLink में आपका स्वागत है",
		"welcome.tagline": "आपका ऑफलाइन मेश यहाँ शुरू होता है।",
		"welcome.local": "स्थानीय रूप से संग्रहीत — कोई अकाउंट नहीं, कोई सर्वर नहीं।",
		"welcome.begin": "शुरू करें", "welcome.skip": "छोड़ें",
		"grp.create": "ग्रुप बनाएं", "grp.name": "ग्रुप का नाम",
		"grp.no_groups": "अभी कोई ग्रुप नहीं",
		"safety.first_aid": "प्राथमिक चिकित्सा संदर्भ",
		"safety.checklists": "उत्तरजीविता चेकलिस्ट",
		"home.hero_eyebrow": "मेश डिस्कवरी",
		"home.scan_title": "डिवाइस स्कैन करें",
		"home.scan_sub1": "रेंज में नोड्स की पहचान करने के लिए एक वाइड-स्पेक्ट्रम मेश ब्रॉडकास्ट शुरू करें।",
		"home.scan_sub2": "एन्क्रिप्टेड डिस्कवरी हैंडशेक स्वचालित है।",
		"home.ready": "तैयार • निष्क्रिय",
		"home.begin_discovery": "डिस्कवरी शुरू करें",
		"home.scanning": "▰▰▰  स्कैन हो रहा है  ▰▰▰",
		"home.found_nodes": "रेंज में %d नोड मिले",
		"home.stat_peers": "पीयर", "home.stat_linked": "लिंक्ड", "home.stat_unread": "अपठित",
		"home.quick_actions": "त्वरित क्रियाएं",
		"home.qa_chat_desc": "निजी लिंक खोलें",
		"home.qa_groups_desc": "व्हाट्सएप-स्टाइल चैनल",
		"home.qa_compass_desc": "दिशा उपकरण",
		"home.qa_safety_desc": "प्राथमिक चिकित्सा और जीवन रक्षा",
		// Safety / First Aid Tags & Titles
		"BLEEDING": "रक्तस्राव", "CPR": "सीपीआर", "BURN": "जलना", "SHOCK": "आघात",
		"COLD": "ठंड", "HEAT": "गर्मी", "CHOKE": "दम घुटना", "BREAK": "फ्रैक्चर",
		"BAG": "बैग", "SHELTER": "आश्रय", "WATER": "पानी", "CONFLICT": "संघर्ष", "QUAKE": "भूकंप",
		"Severe bleeding": "गंभीर रक्तस्राव",
		"CPR (adult, no breathing)": "सीपीआर (वयस्क, सांस न आना)",
		"Burns (thermal)": "जलना (तापीय)",
		"Shock (circulatory)": "आघात (रक्त संचार)",
		"Hypothermia": "हाइपोथर्मिया (अत्यधिक ठंड)",
		"Heat stroke": "लू लगना",
		"Choking (adult, conscious)": "दम घुटना (वयस्क, होश में)",
		"Fracture (suspected)": "फ्रैक्चर (संभावित)",
		"Bug-out bag (24 hr)": "आपातकालीन बैग (24 घंटे)",
		"Shelter selection": "आश्रय का चुनाव",
		"Water purification (no filter)": "जल शुद्धिकरण (बिना फिल्टर)",
		"Conflict / shelling": "संघर्ष / गोलाबारी",
		"Earthquake immediate": "भूकंप के तुरंत बाद",
		// Safety / First Aid Steps
		"Apply direct pressure with cleanest cloth available.": "उपलब्ध सबसे साफ कपड़े से सीधा दबाव डालें।",
		"Elevate the wound above the heart if possible.": "यदि संभव हो तो घाव को हृदय के स्तर से ऊपर उठाएं।",
		"Pack deep wounds — do NOT remove embedded objects.": "गहरे घावों को पैक करें - फंसी हुई वस्तुओं को न निकालें।",
		"If pressure fails on a limb, apply a tourniquet 5 cm above wound, mark TIME on it.": "यदि अंग पर दबाव विफल रहता है, तो घाव से 5 सेमी ऊपर टूर्निकेट लगाएं, उस पर समय (TIME) लिखें।",
		"Treat for shock: keep warm, lay flat, legs raised slightly.": "आघात का इलाज करें: गर्म रखें, सीधा लिटाएं, पैरों को थोड़ा ऊपर उठाएं।",
		"Check responsiveness, shout for help.": "प्रतिक्रिया की जाँच करें, मदद के लिए पुकारें।",
		"30 chest compressions: center of chest, 5–6 cm deep, 100–120/min.": "छाती को 30 बार दबाएं: छाती के बीच में, 5-6 सेमी गहरा, 100-120/मिनट।",
		"2 rescue breaths if trained — otherwise compressions only.": "यदि प्रशिक्षित हैं तो 2 बार सांस दें - अन्यथा केवल छाती दबाएं।",
		"Continue 30:2 cycles until help arrives or person breathes.": "मदद आने या व्यक्ति के सांस लेने तक 30:2 का चक्र जारी रखें।",
		"Do NOT stop unless physically unable.": "शारीरिक रूप से असमर्थ होने तक न रुकें।",
		"Cool with running water (not ice) for at least 20 minutes.": "कम से कम 20 मिनट तक बहते पानी (बर्फ नहीं) से ठंडा करें।",
		"Remove jewelry/clothing near the burn before swelling.": "सूजन आने से पहले जले हुए हिस्से के पास के आभूषण/कपड़े हटा दें।",
		"Cover loosely with sterile non-stick dressing or cling film.": "स्टेराइल नॉन-स्टिक ड्रेसिंग या क्लिंग फिल्म से ढीला ढकें।",
		"Do NOT pop blisters or apply butter/oils.": "फफोले न फोड़ें या मक्खन/तेल न लगाएं।",
		"Seek medical help if larger than the palm or on face/hands/joints.": "यदि हथेली से बड़ा हो या चेहरे/हाथों/जोड़ों पर हो तो चिकित्सकीय सहायता लें।",
		"Lay flat; elevate legs ~30 cm unless head/spine injury suspected.": "सीधा लिटाएं; पैरों को लगभग 30 सेमी ऊपर उठाएं जब तक कि सिर/रीढ़ की चोट का संदेह न हो।",
		"Keep warm with blanket / clothing.": "कंबल/कपड़ों से गर्म रखें।",
		"Loosen tight clothing.": "तंग कपड़े ढीले करें।",
		"Do NOT give food or water.": "भोजन या पानी न दें।",
		"Reassure, monitor breathing & pulse until help arrives.": "भरोसा दें, मदद आने तक सांस और नाड़ी की निगरानी करें।",
		"Move to dry, sheltered spot. Remove wet clothes.": "सूखी, आश्रय वाली जगह पर ले जाएं। गीले कपड़े हटा दें।",
		"Warm core first: blankets, dry layers, body-to-body if available.": "पहले शरीर के मध्य भाग को गर्म करें: कंबल, सूखी परतें, यदि उपलब्ध हो तो शरीर-से-शरीर की गर्मी।",
		"Sugary warm drinks ONLY if fully conscious.": "पूरी तरह होश में होने पर ही मीठे गर्म पेय दें।",
		"Do NOT rub limbs or apply direct intense heat.": "अंगों को न रगड़ें या सीधी तेज गर्मी न दें।",
		"Severe (confused, drowsy): emergency evac required.": "गंभीर स्थिति (भ्रमित, सुस्त): आपातकालीन निकासी आवश्यक है।",
		"Move to coolest available shade.": "उपलब्ध सबसे ठंडी छाया में ले जाएं।",
		"Cool aggressively: wet skin + fanning, ice packs to neck / armpits / groin.": "तेजी से ठंडा करें: गीली त्वचा + पंखा, गर्दन/कांख/जांघों पर आइस पैक।",
		"Sip cool water if conscious & able to swallow.": "यदि होश में हो और निगलने में सक्षम हो तो ठंडा पानी घूंट-घूंट कर पिएं।",
		"Do NOT give meds (paracetamol / aspirin) — ineffective in heat stroke.": "दवाएं (पैरासिटामोल / एस्पिरिन) न दें - लू लगने में बेअसर।",
		"Body temp >40 °C with confusion = medical emergency.": "भ्रम के साथ शरीर का तापमान >40 डिग्री सेल्सियस = आपातकालीन चिकित्सा स्थिति।",
		"Encourage forceful coughing.": "जोर से खांसने के लिए प्रोत्साहित करें।",
		"5 sharp back-blows between shoulder blades.": "कंधे के ब्लेड के बीच 5 तेज थप्पड़ मारें।",
		"5 abdominal thrusts (Heimlich) above navel.": "नाभि के ऊपर 5 बार पेट को दबाएं (हेमलिच)।",
		"Alternate back-blows / thrusts until cleared or unconscious.": "सांस नली साफ होने या बेहोश होने तक बारी-बारी से थप्पड़ और पेट दबाने की क्रिया करें।",
		"If unconscious: begin CPR.": "यदि बेहोश हो जाए: सीपीआर शुरू करें।",
		"Immobilise the limb in the position found.": "अंग को जिस स्थिति में पाया गया है, उसी में स्थिर करें।",
		"Pad and splint with rigid materials, secure above & below break.": "कठोर सामग्री के साथ पैड और स्प्लिंट लगाएं, टूटे हुए हिस्से के ऊपर और नीचे सुरक्षित करें।",
		"Check circulation past the splint (warmth / pulse / colour).": "स्प्लिंट के आगे रक्त संचार की जाँच करें (गर्मी / नाड़ी / रंग)।",
		"Apply cold pack briefly to reduce swelling.": "सूजन कम करने के लिए थोड़ी देर के लिए कोल्ड पैक लगाएं।",
		"Do NOT attempt to realign unless arterial supply is lost.": "जब तक धमनी आपूर्ति न रुक जाए, हड्डियों को सीधा करने का प्रयास न करें।",
		"3 L water + filter / purification tabs": "3 लीटर पानी + फ़िल्टर / शुद्धि टैबलेट",
		"High-cal food (energy bars, jerky)": "उच्च कैलोरी वाला भोजन (एनर्जी बार, जेर्की)",
		"Compact first-aid kit + personal meds (3 days)": "कॉम्पैक्ट फर्स्ट-एड किट + व्यक्तिगत दवाएं (3 दिन)",
		"Multi-tool, sharp knife, paracord 10 m": "मल्टी-टूल, तेज चाकू, पैराकार्ड 10 मीटर",
		"Headlamp + spare batteries / hand-crank torch": "हेडलैंप + अतिरिक्त बैटरी / हैंड-क्रैंक टॉर्च",
		"Emergency blanket, poncho, fire starter (×2)": "आपातकालीन कंबल, पोंचो, आग जलाने का सामान (×2)",
		"Phone power bank, paper map, compass": "फोन पावर बैंक, कागज़ का नक्शा, कम्पास",
		"Cash, ID copy, contact list on paper": "नकद, आईडी की कॉपी, कागज़ पर संपर्क सूची",
		"Whistle, signal mirror, dust mask N95": "सीटी, सिग्नल मिरर, डस्ट मास्क N95",
		"Spare socks, gloves, beanie": "अतिरिक्त मोजे, दस्ताने, टोपी",
		"Above flood line, off ridges (lightning).": "बाढ़ रेखा के ऊपर, लकीरों से दूर (बिजली गिरने का खतरा)।",
		"Out of wind path, near firewood + water.": "हवा के रास्ते से बाहर, जलाऊ लकड़ी और पानी के पास।",
		"NOT in dry creek beds (flash flood).": "सूखे नदी तलों में नहीं (अचानक बाढ़)।",
		"Two exit routes, line of sight to landmarks.": "दो निकास मार्ग, प्रमुख स्थानों के लिए स्पष्ट दृश्यता।",
		"Mark location for rescuers (X / SOS / fire).": "बचाव दल के लिए स्थान चिह्नित करें (X / SOS / आग)।",
		"Settle sediment 1 hr in clean container.": "साफ बर्तन में तलछट को 1 घंटे तक जमने दें।",
		"Boil at full rolling boil ≥ 1 min (≥ 3 min above 2000 m).": "≥ 1 मिनट तक पूरी तरह उबालें (2000 मीटर से ऊपर ≥ 3 मिनट)।",
		"OR 8 drops 5% bleach per litre, wait 30 min.": "या प्रति लीटर 5% ब्लीच की 8 बूंदें डालें, 30 मिनट तक प्रतीक्षा करें।",
		"OR solar (SODIS): clear PET bottle 6 hr in direct sun.": "या सौर विधि (SODIS): सीधी धूप में साफ पीईटी बोतल 6 घंटे तक रखें।",
		"OR iodine 5 drops/L, wait 30 min (avoid for pregnancy/thyroid).": "या प्रति लीटर 5 बूंदें आयोडीन, 30 मिनट तक प्रतीक्षा करें (गर्भावस्था/थायराइड में बचें)।",
		"Stay LOW — basement, interior room, away from windows.": "नीचे रहें - बेसमेंट, आंतरिक कमरा, खिड़कियों से दूर।",
		"Two solid walls between you and outside (\"two walls rule\").": "आपके और बाहर के बीच दो ठोस दीवारें ('दो दीवार का नियम')।",
		"Fill bathtub & containers with water immediately.": "बाथटब और बर्तनों को तुरंत पानी से भर लें।",
		"Charge devices, prepare battery radio for civil defence freqs.": "उपकरणों को चार्ज करें, नागरिक सुरक्षा आवृत्तियों के लिए बैटरी रेडियो तैयार करें।",
		"Mark FIRST AID supplies; assign a triage corner.": "फर्स्ट एड सामग्री को चिह्नित करें; एक ट्राइएज कॉर्नर निर्धारित करें।",
		"Plan evac route: avoid bridges, fuel stations, military installations.": "निकासी मार्ग की योजना बनाएं: पुलों, ईंधन स्टेशनों, सैन्य प्रतिष्ठानों से बचें।",
		"DROP, COVER, HOLD ON under sturdy table.": "मजबूत मेज के नीचे झुकें, ढकें और पकड़ें।",
		"Stay clear of windows, mirrors, heavy furniture.": "खिड़कियों, शीशों, भारी फर्नीचर से दूर रहें।",
		"If outdoors: open ground, away from buildings & power lines.": "यदि बाहर हैं: खुली जगह में, इमारतों और बिजली लाइनों से दूर।",
		"After shaking: check for gas, fire, injuries.": "झटके के बाद: गैस, आग, चोटों की जाँच करें।",
		"Expect aftershocks for 24–72 hr — keep shoes on.": "24-72 घंटे तक आफ्टरशॉक की उम्मीद करें - जूते पहने रहें।",
	},
}
