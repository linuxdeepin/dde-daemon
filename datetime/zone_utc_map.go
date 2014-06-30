package datetime

import . "pkg.linuxdeepin.com/lib/gettext"

type zoneCityInfo struct {
	Zone string
	Desc string
}

var zoneInfos []zoneCityInfo

func initZoneInfos() {
	zoneInfos = []zoneCityInfo{
		zoneCityInfo{"Pacific/Marquesas", "UTC-11 " + Tr("Marquesas")},
		zoneCityInfo{"Pacific/Niue", "UTC-11 " + Tr("Niue")},
		zoneCityInfo{"US/Samoa", "UTC-11 " + Tr("Samoa")},
		zoneCityInfo{"America/Adak", "UTC-10 " + Tr("Adak")},
		zoneCityInfo{"America/Atka", "UTC-10 " + Tr("Atka")},
		zoneCityInfo{"HST", "UTC-10 " + Tr("HST")},
		zoneCityInfo{"Pacific/Honolulu", "UTC-10 " + Tr("Honolulu")},
		zoneCityInfo{"Pacific/Johnston", "UTC-10 " + Tr("Johnston")},
		zoneCityInfo{"Pacific/Rarotonga", "UTC-10 " + Tr("Rarotonga")},
		zoneCityInfo{"Pacific/Tahiti", "UTC-10 " + Tr("Tahiti")},
		zoneCityInfo{"US/Aleutian", "UTC-10 " + Tr("Aleutian")},
		zoneCityInfo{"US/Hawaii", "UTC-10 " + Tr("Hawaii")},
		zoneCityInfo{"America/Anchorage", "UTC-09 " + Tr("Anchorage")},
		zoneCityInfo{"America/Juneau", "UTC-09 " + Tr("Juneau")},
		zoneCityInfo{"America/Nome", "UTC-09 " + Tr("Nome")},
		zoneCityInfo{"America/Sitka", "UTC-09 " + Tr("Sitka")},
		zoneCityInfo{"America/Yakutat", "UTC-09 " + Tr("Yakutat")},
		zoneCityInfo{"Pacific/Gambier", "UTC-09 " + Tr("Gambier")},
		zoneCityInfo{"US/Alaska", "UTC-09 " + Tr("Alaska")},
		zoneCityInfo{"America/Dawson", "UTC-08 " + Tr("Dawson")},
		zoneCityInfo{"America/Ensenada", "UTC-08 " + Tr("Ensenada")},
		zoneCityInfo{"America/Los_Angeles", "UTC-08 " + Tr("Los Angeles")},
		zoneCityInfo{"America/Metlakatla", "UTC-08 " + Tr("Metlakatla")},
		zoneCityInfo{"America/Santa_Isabel", "UTC-08 " + Tr("Santa Isabel")},
		zoneCityInfo{"America/Tijuana", "UTC-08 " + Tr("Tijuana")},
		zoneCityInfo{"America/Vancouver", "UTC-08 " + Tr("Vancouver")},
		zoneCityInfo{"America/Whitehorse", "UTC-08 " + Tr("Whitehorse")},
		zoneCityInfo{"Canada/Newfoundland", "UTC-08 " + Tr("Newfoundland")},
		zoneCityInfo{"Canada/Yukon", "UTC-08 " + Tr("Yukon")},
		zoneCityInfo{"Mexico/BajaNorte", "UTC-08 " + Tr("BajaNorte")},
		zoneCityInfo{"Pacific/Pitcairn", "UTC-08 " + Tr("Pitcairn")},
		zoneCityInfo{"US/Pacific", "UTC-08 " + Tr("Pacific")},
		zoneCityInfo{"US/Pacific-New", "UTC-08 " + Tr("Pacific New")},
		zoneCityInfo{"America/Boise", "UTC-07 " + Tr("Boise")},
		zoneCityInfo{"America/Cambridge_Bay", "UTC-07 " + Tr("Cambridge Bay")},
		zoneCityInfo{"America/Chihuahua", "UTC-07 " + Tr("Chihuahua")},
		zoneCityInfo{"America/Creston", "UTC-07 " + Tr("Creston")},
		zoneCityInfo{"America/Dawson_Creek", "UTC-07 " + Tr("Dawson Creek")},
		zoneCityInfo{"America/Denver", "UTC-07 " + Tr("Denver")},
		zoneCityInfo{"America/Edmonton", "UTC-07 " + Tr("Edmonton")},
		zoneCityInfo{"America/Hermosillo", "UTC-07 " + Tr("Hermosillo")},
		zoneCityInfo{"America/Inuvik", "UTC-07 " + Tr("Inuvik")},
		zoneCityInfo{"America/Mazatlan", "UTC-07 " + Tr("Mazatlan")},
		zoneCityInfo{"America/Ojinaga", "UTC-07 " + Tr("Ojinaga")},
		zoneCityInfo{"America/Phoenix", "UTC-07 " + Tr("Phoenix")},
		zoneCityInfo{"America/Shiprock", "UTC-07 " + Tr("Shiprock")},
		zoneCityInfo{"America/Yellowknife", "UTC-07 " + Tr("Yellowknife")},
		zoneCityInfo{"Canada/Mountain", "UTC-07 " + Tr("Canada Mountain")},
		zoneCityInfo{"Mexico/BajaSur", "UTC-07 " + Tr("BajaSur")},
		zoneCityInfo{"MST", "UTC-07 " + Tr("MST")},
		zoneCityInfo{"Navajo", "UTC-07 " + Tr("Navajo")},
		zoneCityInfo{"US/Arizona", "UTC-07 " + Tr("Arizona")},
		zoneCityInfo{"US/Mountain", "UTC-07 " + Tr("USA Mountain")},
		zoneCityInfo{"America/Bahia_Banderas", "UTC-06 " + Tr("Bahia Banderas")},
		zoneCityInfo{"America/Belize", "UTC-06 " + Tr("Belize")},
		zoneCityInfo{"America/Cancun", "UTC-06 " + Tr("Cancun")},
		zoneCityInfo{"America/Chicago", "UTC-06 " + Tr("Chicago")},
		zoneCityInfo{"America/Costa_Rica", "UTC-06 " + Tr("Costa Rica")},
		zoneCityInfo{"America/El_Salvador", "UTC-06 " + Tr("El Salvador")},
		zoneCityInfo{"America/Guatemala", "UTC-06 " + Tr("Guatemala")},
		zoneCityInfo{"America/Indiana/Knox", "UTC-06 " + Tr("Knox")},
		zoneCityInfo{"America/Indiana/Tell_City", "UTC-06 " + Tr("Tell City")},
		zoneCityInfo{"America/Managua", "UTC-06 " + Tr("Managua")},
		zoneCityInfo{"America/Matamoros", "UTC-06 " + Tr("Matamoros")},
		zoneCityInfo{"America/Menominee", "UTC-06 " + Tr("Menominee")},
		zoneCityInfo{"America/Merida", "UTC-06 " + Tr("Merida")},
		zoneCityInfo{"America/Mexico_City", "UTC-06 " + Tr("Mexico City")},
		zoneCityInfo{"America/Monterrey", "UTC-06 " + Tr("Monterrey")},
		zoneCityInfo{"America/North_Dakota/Beulah", "UTC-06 " + Tr("Beulah")},
		zoneCityInfo{"America/North_Dakota/New_Salem", "UTC-06 " + Tr("New Salem")},
		zoneCityInfo{"America/Rainy_River", "UTC-06 " + Tr("Rainy River")},
		zoneCityInfo{"America/Swift_Current", "UTC-06 " + Tr("Swift Current")},
		zoneCityInfo{"America/Regina", "UTC-06 " + Tr("Regina")},
		zoneCityInfo{"America/Resolute", "UTC-06 " + Tr("Resolute")},
		zoneCityInfo{"America/Tegucigalpa", "UTC-06 " + Tr("Tegucigalpa")},
		zoneCityInfo{"America/Winnipeg", "UTC-06 " + Tr("Winnipeg")},
		zoneCityInfo{"Canada/East-Saskatchewan", "UTC-06 " + Tr("East Saskatchewan")},
		zoneCityInfo{"Canada/Saskatchewan", "UTC-06 " + Tr("Saskatchewan")},
		zoneCityInfo{"Chile/EasterIsland", "UTC-06 " + Tr("EasterIsland")},
		zoneCityInfo{"Mexico/General", "UTC-06 " + Tr("General")},
		zoneCityInfo{"Pacific/Easter", "UTC-06 " + Tr("Pacific East")},
		zoneCityInfo{"Pacific/Galapagos", "UTC-06 " + Tr("Galapagos")},
		zoneCityInfo{"US/Central", "UTC-06 " + Tr("USA Central")},
		zoneCityInfo{"US/Indiana-Starke", "UTC-06 " + Tr("Indiana Starke")},
		zoneCityInfo{"America/Atikokan", "UTC-05 " + Tr("Atikokan")},
		zoneCityInfo{"America/Bogota", "UTC-05 " + Tr("Bogota")},
		zoneCityInfo{"America/Cayman", "UTC-05 " + Tr("Cayman")},
		zoneCityInfo{"America/Coral_Harbour", "UTC-05 " + Tr("Coral Harbour")},
		zoneCityInfo{"America/Detroit", "UTC-05 " + Tr("Detroit")},
		zoneCityInfo{"America/Fort_Wayne", "UTC-05 " + Tr("Fort Wayne")},
		zoneCityInfo{"America/Grand_Turk", "UTC-05 " + Tr("Grand Turk")},
		zoneCityInfo{"America/Guayaquil", "UTC-05 " + Tr("Guayaquil")},
		zoneCityInfo{"America/Havana", "UTC-05 " + Tr("Havana")},
		zoneCityInfo{"America/Indiana/Marengo", "UTC-05 " + Tr("Marengo")},
		zoneCityInfo{"America/Indiana/Petersburg", "UTC-05 " + Tr("Petersburg")},
		zoneCityInfo{"America/Indiana/Vevay", "UTC-05 " + Tr("Vevay")},
		zoneCityInfo{"America/Indiana/Vincennes", "UTC-05 " + Tr("Vincennes")},
		zoneCityInfo{"America/Indiana/Winamac", "UTC-05 " + Tr("Winamac")},
		zoneCityInfo{"America/Indianapolis", "UTC-05 " + Tr("Indianapolis")},
		zoneCityInfo{"America/Iqaluit", "UTC-05 " + Tr("Iqaluit")},
		zoneCityInfo{"America/Jamaica", "UTC-05 " + Tr("Jamaica")},
		zoneCityInfo{"America/Lima", "UTC-05 " + Tr("Lima")},
		zoneCityInfo{"America/Louisville", "UTC-05 " + Tr("Louisville")},
		zoneCityInfo{"America/Kentucky/Monticello", "UTC-05 " + Tr("Monticello")},
		zoneCityInfo{"America/Montreal", "UTC-05 " + Tr("Montreal")},
		zoneCityInfo{"America/Nassau", "UTC-05 " + Tr("Nassau")},
		zoneCityInfo{"America/New_York", "UTC-05 " + Tr("New York")},
		zoneCityInfo{"America/Nipigon", "UTC-05 " + Tr("Nipigon")},
		zoneCityInfo{"America/Panama", "UTC-05 " + Tr("Panama")},
		zoneCityInfo{"America/Port-au-Prince", "UTC-05 " + Tr("Port au Prince")},
		zoneCityInfo{"America/Porto_Acre", "UTC-05 " + Tr("Porto Acre")},
		zoneCityInfo{"America/Rio_Branco", "UTC-05 " + Tr("Rio Branco")},
		zoneCityInfo{"America/Thunder_Bay", "UTC-05 " + Tr("Thunder Bay")},
		zoneCityInfo{"America/Toronto", "UTC-05 " + Tr("Toronto")},
		zoneCityInfo{"Australia/Yancowinna", "UTC-05 " + Tr("Yancowinna")},
		zoneCityInfo{"Canada/Eastern", "UTC-05 " + Tr("Canada Eastern")},
		zoneCityInfo{"Cuba", "UTC-05 " + Tr("Cuba")},
		zoneCityInfo{"EST", "UTC-05 " + Tr("EST")},
		zoneCityInfo{"US/Eastern", "UTC-05 " + Tr("USA Eastern")},
		zoneCityInfo{"US/East-Indiana", "UTC-05 " + Tr("East Indiana")},
		zoneCityInfo{"US/Michigan", "UTC-05 " + Tr("Michigan")},
		zoneCityInfo{"America/Anguilla", "UTC-04 " + Tr("Anguilla")},
		zoneCityInfo{"America/Antigua", "UTC-04 " + Tr("Antigua")},
		zoneCityInfo{"America/Aruba", "UTC-04 " + Tr("Aruba")},
		zoneCityInfo{"America/Asuncion", "UTC-04 " + Tr("Asuncion")},
		zoneCityInfo{"America/Barbados", "UTC-04 " + Tr("Barbados")},
		zoneCityInfo{"America/Boa_Vista", "UTC-04 " + Tr("Boa Vista")},
		zoneCityInfo{"America/Campo_Grande", "UTC-04 " + Tr("Campo Grande")},
		zoneCityInfo{"America/Cuiaba", "UTC-04 " + Tr("Cuiaba")},
		zoneCityInfo{"America/Curacao", "UTC-04 " + Tr("Curacao")},
		zoneCityInfo{"America/Dominica", "UTC-04 " + Tr("Dominica")},
		zoneCityInfo{"America/Eirunepe", "UTC-04 " + Tr("Eirunepe")},
		zoneCityInfo{"America/Glace_Bay", "UTC-04 " + Tr("Glace Bay")},
		zoneCityInfo{"America/Goose_Bay", "UTC-04 " + Tr("Goose Bay")},
		zoneCityInfo{"America/Grenada", "UTC-04 " + Tr("Grenada")},
		zoneCityInfo{"America/Guadeloupe", "UTC-04 " + Tr("Guadeloupe")},
		zoneCityInfo{"America/Guyana", "UTC-04 " + Tr("Guyana")},
		zoneCityInfo{"America/Halifax", "UTC-04 " + Tr("Halifax")},
		zoneCityInfo{"America/Kralendijk", "UTC-04 " + Tr("Kralendijk")},
		zoneCityInfo{"America/La_Paz", "UTC-04 " + Tr("La Paz")},
		zoneCityInfo{"America/Lower_Princes", "UTC-04 " + Tr("Lower Princes")},
		zoneCityInfo{"America/Manaus", "UTC-04 " + Tr("Manaus")},
		zoneCityInfo{"America/Marigot", "UTC-04 " + Tr("Marigot")},
		zoneCityInfo{"America/Martinique", "UTC-04 " + Tr("Martinique")},
		zoneCityInfo{"America/Moncton", "UTC-04 " + Tr("Moncton")},
		zoneCityInfo{"America/Montserrat", "UTC-04 " + Tr("Montserrat")},
		zoneCityInfo{"America/Port_of_Spain", "UTC-04 " + Tr("Port of Spain")},
		zoneCityInfo{"America/Porto_Velho", "UTC-04 " + Tr("Porto Velho")},
		zoneCityInfo{"America/Puerto_Rico", "UTC-04 " + Tr("Puerto Rico")},
		zoneCityInfo{"America/Santiago", "UTC-04 " + Tr("Santiago")},
		zoneCityInfo{"America/Santo_Domingo", "UTC-04 " + Tr("Santo Domingo")},
		zoneCityInfo{"America/St_Barthelemy", "UTC-04 " + Tr("St Barthelemy")},
		zoneCityInfo{"America/St_Johns", "UTC-04 " + Tr("St Johns")},
		zoneCityInfo{"America/St_Lucia", "UTC-04 " + Tr("St Lucia")},
		zoneCityInfo{"America/St_Thomas", "UTC-04 " + Tr("St Thomas")},
		zoneCityInfo{"America/St_Vincent", "UTC-04 " + Tr("St Vincent")},
		zoneCityInfo{"America/Thule", "UTC-04 " + Tr("Thule")},
		zoneCityInfo{"America/Tortola", "UTC-04 " + Tr("Tortola")},
		zoneCityInfo{"America/Virgin", "UTC-04 " + Tr("Virgin")},
		zoneCityInfo{"Antarctica/Palmer", "UTC-04 " + Tr("Palmer")},
		zoneCityInfo{"Atlantic/Bermuda", "UTC-04 " + Tr("Bermuda")},
		zoneCityInfo{"Brazil/West", "UTC-04 " + Tr("Brazil West")},
		zoneCityInfo{"Canada/Atlantic", "UTC-04 " + Tr("Atlantic")},
		zoneCityInfo{"Chile/Continental", "UTC-04 " + Tr("Continental")},
		zoneCityInfo{"America/Araguaina", "UTC-03 " + Tr("Araguaina")},
		zoneCityInfo{"America/Argentina/Catamarca", "UTC-03 " + Tr("Catamarca")},
		zoneCityInfo{"America/Argentina/ComodRivadavia", "UTC-03 " + Tr("ComodRivadavia")},
		zoneCityInfo{"America/Argentina/Jujuy", "UTC-03 " + Tr("Jujuy")},
		zoneCityInfo{"America/Argentina/La_Rioja", "UTC-03 " + Tr("La Rioja")},
		zoneCityInfo{"America/Argentina/Rio_Gallegos", "UTC-03 " + Tr("Rio Gallegos")},
		zoneCityInfo{"America/Argentina/Salta", "UTC-03 " + Tr("Salta")},
		zoneCityInfo{"America/Argentina/San_Juan", "UTC-03 " + Tr("San Juan")},
		zoneCityInfo{"America/Argentina/San_Luis", "UTC-03 " + Tr("San Luis")},
		zoneCityInfo{"America/Argentina/Tucuman", "UTC-03 " + Tr("Tucuman")},
		zoneCityInfo{"America/Argentina/Ushuaia", "UTC-03 " + Tr("Ushuaia")},
		zoneCityInfo{"America/Bahia", "UTC-03 " + Tr("Bahia")},
		zoneCityInfo{"America/Belem", "UTC-03 " + Tr("Belem")},
		zoneCityInfo{"America/Caracas", "UTC-03 " + Tr("Caracas")},
		zoneCityInfo{"America/Cayenne", "UTC-03 " + Tr("Cayenne")},
		zoneCityInfo{"America/Cordoba", "UTC-03 " + Tr("Cordoba")},
		zoneCityInfo{"America/Fortaleza", "UTC-03 " + Tr("Fortaleza")},
		zoneCityInfo{"America/Godthab", "UTC-03 " + Tr("Godthab")},
		zoneCityInfo{"America/Maceio", "UTC-03 " + Tr("Maceio")},
		zoneCityInfo{"America/Mendoza", "UTC-03 " + Tr("Mendoza")},
		zoneCityInfo{"America/Miquelon", "UTC-03 " + Tr("Miquelon")},
		zoneCityInfo{"America/Montevideo", "UTC-03 " + Tr("Montevideo")},
		zoneCityInfo{"America/Paramaribo", "UTC-03 " + Tr("Paramaribo")},
		zoneCityInfo{"America/Recife", "UTC-03 " + Tr("Recife")},
		zoneCityInfo{"America/Rosario", "UTC-03 " + Tr("Rosario")},
		zoneCityInfo{"America/Santarem", "UTC-03 " + Tr("Santarem")},
		zoneCityInfo{"America/Sao_Paulo", "UTC-03 " + Tr("Sao Paulo")},
		zoneCityInfo{"Antarctica/Rothera", "UTC-03 " + Tr("Rothera")},
		zoneCityInfo{"Atlantic/Stanley", "UTC-03 " + Tr("Stanley")},
		zoneCityInfo{"Brazil/East", "UTC-03 " + Tr("Brazil East")},
		zoneCityInfo{"America/Noronha", "UTC-02 " + Tr("Noronha")},
		zoneCityInfo{"Atlantic/South_Georgia", "UTC-02 " + Tr("South Georgia")},
		zoneCityInfo{"Atlantic/Cape_Verde", "UTC-01 " + Tr("Cape Verde")},
		zoneCityInfo{"Africa/Abidjan", "UTC+00 " + Tr("Abidjan")},
		zoneCityInfo{"Africa/Accra", "UTC+00 " + Tr("Accra")},
		zoneCityInfo{"Africa/Bamako", "UTC+00 " + Tr("Bamako")},
		zoneCityInfo{"Africa/Banjul", "UTC+00 " + Tr("Banjul")},
		zoneCityInfo{"Africa/Bissau", "UTC+00 " + Tr("Bissau")},
		zoneCityInfo{"Africa/Casablanca", "UTC+00 " + Tr("Casablanca")},
		zoneCityInfo{"Africa/Conakry", "UTC+00 " + Tr("Conakry")},
		zoneCityInfo{"Africa/Dakar", "UTC+00 " + Tr("Dakar")},
		zoneCityInfo{"Africa/El_Aaiun", "UTC+00 " + Tr("El Aaiun")},
		zoneCityInfo{"Africa/Freetown", "UTC+00 " + Tr("Freetown")},
		zoneCityInfo{"Africa/Lome", "UTC+00 " + Tr("Lome")},
		zoneCityInfo{"Africa/Monrovia", "UTC+00 " + Tr("Monrovia")},
		zoneCityInfo{"Africa/Nouakchott", "UTC+00 " + Tr("Nouakchott")},
		zoneCityInfo{"Africa/Ouagadougou", "UTC+00 " + Tr("Ouagadougou")},
		zoneCityInfo{"Africa/Sao_Tome", "UTC+00 " + Tr("Sao Tome")},
		zoneCityInfo{"Africa/Timbuktu", "UTC+00 " + Tr("Timbuktu")},
		zoneCityInfo{"Atlantic/Canary", "UTC+00 " + Tr("Canary")},
		zoneCityInfo{"Atlantic/Madeira", "UTC+00 " + Tr("Madeira")},
		zoneCityInfo{"Atlantic/Reykjavik", "UTC+00 " + Tr("Reykjavik")},
		zoneCityInfo{"Atlantic/St_Helena", "UTC+00 " + Tr("St Helena")},
		zoneCityInfo{"Eire", "UTC+00 " + Tr("Eire")},
		zoneCityInfo{"Europe/Belfast", "UTC+00 " + Tr("Belfast")},
		zoneCityInfo{"Europe/Dublin", "UTC+00 " + Tr("Dublin")},
		zoneCityInfo{"Europe/Guernsey", "UTC+00 " + Tr("Guernsey")},
		zoneCityInfo{"Europe/Isle_of_Man", "UTC+00 " + Tr("Isle of Man")},
		zoneCityInfo{"Europe/Jersey", "UTC+00 " + Tr("Jersey")},
		zoneCityInfo{"Europe/Lisbon", "UTC+00 " + Tr("Lisbon")},
		zoneCityInfo{"Europe/London", "UTC+00 " + Tr("London")},
		zoneCityInfo{"GB", "UTC+00 " + Tr("GB")},
		zoneCityInfo{"GB-Eire", "UTC+00 " + Tr("GB Eire")},
		zoneCityInfo{"GMT", "UTC+00 " + Tr("USA GMT")},
		zoneCityInfo{"GMT+0", "UTC+00 " + Tr("Brazil GMT")},
		zoneCityInfo{"Greenwich", "UTC+00 " + Tr("Greenwich")},
		zoneCityInfo{"Iceland", "UTC+00 " + Tr("Iceland")},
		zoneCityInfo{"Portugal", "UTC+00 " + Tr("Portugal")},
		zoneCityInfo{"UCT", "UTC+00 " + Tr("UCT")},
		zoneCityInfo{"Universal", "UTC+00 " + Tr("Universal")},
		zoneCityInfo{"UTC", "UTC+00 " + Tr("UTC")},
		zoneCityInfo{"WET", "UTC+00 " + Tr("WET")},
		zoneCityInfo{"Zulu", "UTC+00 " + Tr("Zulu")},
		zoneCityInfo{"Africa/Algiers", "UTC+01 " + Tr("Algiers")},
		zoneCityInfo{"Africa/Bangui", "UTC+01 " + Tr("Bangui")},
		zoneCityInfo{"Africa/Brazzaville", "UTC+01 " + Tr("Brazzaville")},
		zoneCityInfo{"Africa/Ceuta", "UTC+01 " + Tr("Ceuta")},
		zoneCityInfo{"Africa/Douala", "UTC+01 " + Tr("Douala")},
		zoneCityInfo{"Africa/Kinshasa", "UTC+01 " + Tr("Kinshasa")},
		zoneCityInfo{"Africa/Lagos", "UTC+01 " + Tr("Lagos")},
		zoneCityInfo{"Africa/Libreville", "UTC+01 " + Tr("Libreville")},
		zoneCityInfo{"Africa/Luanda", "UTC+01 " + Tr("Luanda")},
		zoneCityInfo{"Africa/Malabo", "UTC+01 " + Tr("Malabo")},
		zoneCityInfo{"Africa/Ndjamena", "UTC+01 " + Tr("Ndjamena")},
		zoneCityInfo{"Africa/Niamey", "UTC+01 " + Tr("Niamey")},
		zoneCityInfo{"Africa/Porto-Novo", "UTC+01 " + Tr("Porto Novo")},
		zoneCityInfo{"Africa/Tunis", "UTC+01 " + Tr("Tunis")},
		zoneCityInfo{"Africa/Windhoek", "UTC+01 " + Tr("Windhoek")},
		zoneCityInfo{"Arctic/Longyearbyen", "UTC+01 " + Tr("Longyearbyen")},
		zoneCityInfo{"CET", "UTC+01 " + Tr("CET")},
		zoneCityInfo{"Europe/Amsterdam", "UTC+01 " + Tr("Amsterdam")},
		zoneCityInfo{"Europe/Andorra", "UTC+01 " + Tr("Andorra")},
		zoneCityInfo{"Europe/Belgrade", "UTC+01 " + Tr("Belgrade")},
		zoneCityInfo{"Europe/Berlin", "UTC+01 " + Tr("Berlin")},
		zoneCityInfo{"Europe/Bratislava", "UTC+01 " + Tr("Bratislava")},
		zoneCityInfo{"Europe/Brussels", "UTC+01 " + Tr("Brussels")},
		zoneCityInfo{"Europe/Budapest", "UTC+01 " + Tr("Budapest")},
		zoneCityInfo{"Europe/Copenhagen", "UTC+01 " + Tr("Copenhagen")},
		zoneCityInfo{"Europe/Gibraltar", "UTC+01 " + Tr("Gibraltar")},
		zoneCityInfo{"Europe/Ljubljana", "UTC+01 " + Tr("Ljubljana")},
		zoneCityInfo{"Europe/Luxembourg", "UTC+01 " + Tr("Luxembourg")},
		zoneCityInfo{"Europe/Madrid", "UTC+01 " + Tr("Madrid")},
		zoneCityInfo{"Europe/Malta", "UTC+01 " + Tr("Malta")},
		zoneCityInfo{"Europe/Monaco", "UTC+01 " + Tr("Monaco")},
		zoneCityInfo{"Europe/Oslo", "UTC+01 " + Tr("Oslo")},
		zoneCityInfo{"Europe/Paris", "UTC+01 " + Tr("Paris")},
		zoneCityInfo{"Europe/Podgorica", "UTC+01 " + Tr("Podgorica")},
		zoneCityInfo{"Europe/Prague", "UTC+01 " + Tr("Prague")},
		zoneCityInfo{"Europe/Rome", "UTC+01 " + Tr("Rome")},
		zoneCityInfo{"Europe/San_Marino", "UTC+01 " + Tr("San Marino")},
		zoneCityInfo{"Europe/Sarajevo", "UTC+01 " + Tr("Sarajevo")},
		zoneCityInfo{"Europe/Skopje", "UTC+01 " + Tr("Skopje")},
		zoneCityInfo{"Europe/Stockholm", "UTC+01 " + Tr("Stockholm")},
		zoneCityInfo{"Europe/Tirane", "UTC+01 " + Tr("Tirane")},
		zoneCityInfo{"Europe/Vaduz", "UTC+01 " + Tr("Vaduz")},
		zoneCityInfo{"Europe/Vatican", "UTC+01 " + Tr("Vatican")},
		zoneCityInfo{"Europe/Vienna", "UTC+01 " + Tr("Vienna")},
		zoneCityInfo{"Europe/Warsaw", "UTC+01 " + Tr("Warsaw")},
		zoneCityInfo{"Europe/Zagreb", "UTC+01 " + Tr("Zagreb")},
		zoneCityInfo{"Europe/Zurich", "UTC+01 " + Tr("Zurich")},
		zoneCityInfo{"MET", "UTC+01 " + Tr("MET")},
		zoneCityInfo{"Africa/Blantyre", "UTC+02 " + Tr("Blantyre")},
		zoneCityInfo{"Africa/Bujumbura", "UTC+02 " + Tr("Bujumbura")},
		zoneCityInfo{"Africa/Cairo", "UTC+02 " + Tr("Cairo")},
		zoneCityInfo{"Africa/Gaborone", "UTC+02 " + Tr("Gaborone")},
		zoneCityInfo{"Africa/Harare", "UTC+02 " + Tr("Harare")},
		zoneCityInfo{"Africa/Johannesburg", "UTC+02 " + Tr("Johannesburg")},
		zoneCityInfo{"Africa/Kigali", "UTC+02 " + Tr("Kigali")},
		zoneCityInfo{"Africa/Lubumbashi", "UTC+02 " + Tr("Lubumbashi")},
		zoneCityInfo{"Africa/Lusaka", "UTC+02 " + Tr("Lusaka")},
		zoneCityInfo{"Africa/Maputo", "UTC+02 " + Tr("Maputo")},
		zoneCityInfo{"Africa/Maseru", "UTC+02 " + Tr("Maseru")},
		zoneCityInfo{"Africa/Mbabane", "UTC+02 " + Tr("Mbabane")},
		zoneCityInfo{"Africa/Tripoli", "UTC+02 " + Tr("Tripoli")},
		zoneCityInfo{"Asia/Amman", "UTC+02 " + Tr("Amman")},
		zoneCityInfo{"Asia/Beirut", "UTC+02 " + Tr("Beirut")},
		zoneCityInfo{"Asia/Damascus", "UTC+02 " + Tr("Damascus")},
		zoneCityInfo{"Asia/Gaza", "UTC+02 " + Tr("Gaza")},
		zoneCityInfo{"Asia/Hebron", "UTC+02 " + Tr("Hebron")},
		zoneCityInfo{"Asia/Jerusalem", "UTC+02 " + Tr("Jerusalem")},
		zoneCityInfo{"Asia/Tehran", "UTC+02 " + Tr("Tehran")},
		zoneCityInfo{"EET", "UTC+02 " + Tr("EET")},
		zoneCityInfo{"Egypt", "UTC+02 " + Tr("Egypt")},
		zoneCityInfo{"Europe/Athens", "UTC+02 " + Tr("Athens")},
		zoneCityInfo{"Europe/Bucharest", "UTC+02 " + Tr("Bucharest")},
		zoneCityInfo{"Europe/Chisinau", "UTC+02 " + Tr("Chisinau")},
		zoneCityInfo{"Europe/Helsinki", "UTC+02 " + Tr("Helsinki")},
		zoneCityInfo{"Europe/Istanbul", "UTC+02 " + Tr("Istanbul")},
		zoneCityInfo{"Europe/Kiev", "UTC+02 " + Tr("Kiev")},
		zoneCityInfo{"Europe/Mariehamn", "UTC+02 " + Tr("Mariehamn")},
		zoneCityInfo{"Europe/Nicosia", "UTC+02 " + Tr("Nicosia")},
		zoneCityInfo{"Europe/Riga", "UTC+02 " + Tr("Riga")},
		zoneCityInfo{"Europe/Simferopol", "UTC+02 " + Tr("Simferopol")},
		zoneCityInfo{"Europe/Sofia", "UTC+02 " + Tr("Sofia")},
		zoneCityInfo{"Europe/Tallinn", "UTC+02 " + Tr("Tallinn")},
		zoneCityInfo{"Europe/Uzhgorod", "UTC+02 " + Tr("Uzhgorod")},
		zoneCityInfo{"Europe/Vilnius", "UTC+02 " + Tr("Vilnius")},
		zoneCityInfo{"Europe/Zaporozhye", "UTC+02 " + Tr("Zaporozhye")},
		zoneCityInfo{"Iran", "UTC+02 " + Tr("Iran")},
		zoneCityInfo{"Libya", "UTC+02 " + Tr("Libya")},
		zoneCityInfo{"Turkey", "UTC+02 " + Tr("Turkey")},
		zoneCityInfo{"Africa/Addis_Ababa", "UTC+03 " + Tr("Addis Ababa")},
		zoneCityInfo{"Africa/Asmara", "UTC+03 " + Tr("Asmara")},
		zoneCityInfo{"Africa/Dar_es_Salaam", "UTC+03 " + Tr("Dar es Salaam")},
		zoneCityInfo{"Africa/Djibouti", "UTC+03 " + Tr("Djibouti")},
		zoneCityInfo{"Africa/Juba", "UTC+03 " + Tr("Juba")},
		zoneCityInfo{"Africa/Kampala", "UTC+03 " + Tr("Kampala")},
		zoneCityInfo{"Africa/Khartoum", "UTC+03 " + Tr("Khartoum")},
		zoneCityInfo{"Africa/Mogadishu", "UTC+03 " + Tr("Mogadishu")},
		zoneCityInfo{"Africa/Nairobi", "UTC+03 " + Tr("Nairobi")},
		zoneCityInfo{"Asia/Aden", "UTC+03 " + Tr("Aden")},
		zoneCityInfo{"Asia/Baghdad", "UTC+03 " + Tr("Baghdad")},
		zoneCityInfo{"Asia/Bahrain", "UTC+03 " + Tr("Bahrain")},
		zoneCityInfo{"Asia/Kuwait", "UTC+03 " + Tr("Kuwait")},
		zoneCityInfo{"Asia/Rangoon", "UTC+03 " + Tr("Rangoon")},
		zoneCityInfo{"Asia/Qatar", "UTC+03 " + Tr("Qatar")},
		zoneCityInfo{"Europe/Kaliningrad", "UTC+03 " + Tr("Kaliningrad")},
		zoneCityInfo{"Europe/Minsk", "UTC+03 " + Tr("Minsk")},
		zoneCityInfo{"Indian/Antananarivo", "UTC+03 " + Tr("Antananarivo")},
		zoneCityInfo{"Asia/Baku", "UTC+04 " + Tr("Baku")},
		zoneCityInfo{"Asia/Dubai", "UTC+04 " + Tr("Dubai")},
		zoneCityInfo{"Asia/Muscat", "UTC+04 " + Tr("Muscat")},
		zoneCityInfo{"Asia/Tbilisi", "UTC+04 " + Tr("Tbilisi")},
		zoneCityInfo{"Asia/Yerevan", "UTC+04 " + Tr("Yerevan")},
		zoneCityInfo{"Europe/Moscow", "UTC+04 " + Tr("Moscow")},
		zoneCityInfo{"Europe/Samara", "UTC+04 " + Tr("Samara")},
		zoneCityInfo{"Europe/Volgograd", "UTC+04 " + Tr("Volgograd")},
		zoneCityInfo{"Indian/Mahe", "UTC+04 " + Tr("Mahe")},
		zoneCityInfo{"Indian/Mauritius", "UTC+04 " + Tr("Mauritius")},
		zoneCityInfo{"Indian/Reunion", "UTC+04 " + Tr("Reunion")},
		zoneCityInfo{"Antarctica/Davis", "UTC+05 " + Tr("Davis")},
		zoneCityInfo{"Antarctica/Mawson", "UTC+05 " + Tr("Mawson")},
		zoneCityInfo{"Asia/Aqtau", "UTC+05 " + Tr("Aqtau")},
		zoneCityInfo{"Asia/Aqtobe", "UTC+05 " + Tr("Aqtobe")},
		zoneCityInfo{"Asia/Ashkhabad", "UTC+05 " + Tr("Ashkhabad")},
		zoneCityInfo{"Asia/Dushanbe", "UTC+05 " + Tr("Dushanbe")},
		zoneCityInfo{"Asia/Karachi", "UTC+05 " + Tr("Karachi")},
		zoneCityInfo{"Asia/Oral", "UTC+05 " + Tr("Oral")},
		zoneCityInfo{"Asia/Samarkand", "UTC+05 " + Tr("Samarkand")},
		zoneCityInfo{"Asia/Tashkent", "UTC+05 " + Tr("Tashkent")},
		zoneCityInfo{"Indian/Kerguelen", "UTC+05 " + Tr("Kerguelen")},
		zoneCityInfo{"Indian/Maldives", "UTC+05 " + Tr("Maldives")},
		zoneCityInfo{"Antarctica/Vostok", "UTC+06 " + Tr("Vostok")},
		zoneCityInfo{"Asia/Almaty", "UTC+06 " + Tr("Almaty")},
		zoneCityInfo{"Asia/Bishkek", "UTC+06 " + Tr("Bishkek")},
		zoneCityInfo{"Asia/Colombo", "UTC+06 " + Tr("Colombo")},
		zoneCityInfo{"Asia/Dhaka", "UTC+06 " + Tr("Dhaka")},
		zoneCityInfo{"Asia/Qyzylorda", "UTC+06 " + Tr("Qyzylorda")},
		zoneCityInfo{"Asia/Thimphu", "UTC+06 " + Tr("Thimphu")},
		zoneCityInfo{"Asia/Yekaterinburg", "UTC+06 " + Tr("Yekaterinburg")},
		zoneCityInfo{"Indian/Chagos", "UTC+06 " + Tr("Chagos")},
		zoneCityInfo{"Asia/Bangkok", "UTC+07 " + Tr("Bangkok")},
		zoneCityInfo{"Asia/Ho_Chi_Minh", "UTC+07 " + Tr("Ho Chi Minh")},
		zoneCityInfo{"Asia/Hovd", "UTC+07 " + Tr("Hovd")},
		zoneCityInfo{"Asia/Jakarta", "UTC+07 " + Tr("Jakarta")},
		zoneCityInfo{"Asia/Novokuznetsk", "UTC+07 " + Tr("Novokuznetsk")},
		zoneCityInfo{"Asia/Novosibirsk", "UTC+07 " + Tr("Novosibirsk")},
		zoneCityInfo{"Asia/Omsk", "UTC+07 " + Tr("Omsk")},
		zoneCityInfo{"Asia/Phnom_Penh", "UTC+07 " + Tr("Phnom Penh")},
		zoneCityInfo{"Asia/Pontianak", "UTC+07 " + Tr("Pontianak")},
		zoneCityInfo{"Asia/Saigon", "UTC+07 " + Tr("Saigon")},
		zoneCityInfo{"Asia/Vientiane", "UTC+07 " + Tr("Vientiane")},
		zoneCityInfo{"Indian/Christmas", "UTC+07 " + Tr("Christmas")},
		zoneCityInfo{"Asia/Brunei", "UTC+08 " + Tr("Brunei")},
		zoneCityInfo{"Asia/Calcutta", "UTC+08 " + Tr("Calcutta")},
		zoneCityInfo{"Asia/Chongqing", "UTC+08 " + Tr("Chongqing")},
		zoneCityInfo{"Asia/Harbin", "UTC+08 " + Tr("Harbin")},
		zoneCityInfo{"Asia/Hong_Kong", "UTC+08 " + Tr("Hong Kong")},
		zoneCityInfo{"Asia/Kashgar", "UTC+08 " + Tr("Kashgar")},
		zoneCityInfo{"Asia/Kathmandu", "UTC+08 " + Tr("Kathmandu")},
		zoneCityInfo{"Asia/Kuala_Lumpur", "UTC+08 " + Tr("Kuala Lumpur")},
		zoneCityInfo{"Asia/Kuching", "UTC+08 " + Tr("Kuching")},
		zoneCityInfo{"Asia/Macao", "UTC+08 " + Tr("Macao")},
		zoneCityInfo{"Asia/Makassar", "UTC+08 " + Tr("Makassar")},
		zoneCityInfo{"Asia/Manila", "UTC+08 " + Tr("Manila")},
		zoneCityInfo{"Asia/Shanghai", "UTC+08 " + Tr("Shanghai")},
		zoneCityInfo{"Asia/Shanghai", "UTC+08 " + Tr("Beijing")},
		zoneCityInfo{"Asia/Singapore", "UTC+08 " + Tr("Singapore")},
		zoneCityInfo{"Asia/Taipei", "UTC+08 " + Tr("Taipei")},
		zoneCityInfo{"Asia/Ujung_Pandang", "UTC+08 " + Tr("Ujung Pandang")},
		zoneCityInfo{"Asia/Ulan_Bator", "UTC+08 " + Tr("Ulan Bator")},
		zoneCityInfo{"Asia/Urumqi", "UTC+08 " + Tr("Urumqi")},
		zoneCityInfo{"Australia/Perth", "UTC+08 " + Tr("Perth")},
		zoneCityInfo{"Australia/West", "UTC+08 " + Tr("Australia West")},
		zoneCityInfo{"Asia/Dili", "UTC+09 " + Tr("Dili")},
		zoneCityInfo{"Asia/Irkutsk", "UTC+09 " + Tr("Irkutsk")},
		zoneCityInfo{"Asia/Jayapura", "UTC+09 " + Tr("Jayapura")},
		zoneCityInfo{"Asia/Pyongyang", "UTC+09 " + Tr("Pyongyang")},
		zoneCityInfo{"Asia/Seoul", "UTC+09 " + Tr("Seoul")},
		zoneCityInfo{"Asia/Tokyo", "UTC+09 " + Tr("Tokyo")},
		zoneCityInfo{"Pacific/Palau", "UTC+09 " + Tr("Palau")},
		zoneCityInfo{"ROK", "UTC+09 " + Tr("ROK")},
		zoneCityInfo{"Antarctica/DumontDUrville", "UTC+10 " + Tr("DumontDUrville")},
		zoneCityInfo{"Asia/Yakutsk", "UTC+10 " + Tr("Yakutsk")},
		zoneCityInfo{"Australia/ACT", "UTC+10 " + Tr("ACT")},
		zoneCityInfo{"Australia/Adelaide", "UTC+10 " + Tr("Adelaide")},
		zoneCityInfo{"Australia/Broken_Hill", "UTC+10 " + Tr("Broken Hill")},
		zoneCityInfo{"Australia/Currie", "UTC+10 " + Tr("Currie")},
		zoneCityInfo{"Australia/Darwin", "UTC+10 " + Tr("Darwin")},
		zoneCityInfo{"Australia/Lindeman", "UTC+10 " + Tr("Lindeman")},
		zoneCityInfo{"Australia/Melbourne", "UTC+10 " + Tr("Melbourne")},
		zoneCityInfo{"Australia/North", "UTC+10 " + Tr("Australia North")},
		zoneCityInfo{"Australia/Queensland", "UTC+10 " + Tr("Queensland")},
		zoneCityInfo{"Australia/South", "UTC+10 " + Tr("Australia South")},
		zoneCityInfo{"Australia/Tasmania", "UTC+10 " + Tr("Tasmania")},
		zoneCityInfo{"Australia/Victoria", "UTC+10 " + Tr("Victoria")},
		zoneCityInfo{"Pacific/Chatham", "UTC+10 " + Tr("Chatham")},
		zoneCityInfo{"Pacific/Guam", "UTC+10 " + Tr("Guam")},
		zoneCityInfo{"Pacific/Port_Moresby", "UTC+10 " + Tr("Port Moresby")},
		zoneCityInfo{"Pacific/Saipan", "UTC+10 " + Tr("Saipan")},
		zoneCityInfo{"Pacific/Truk", "UTC+10 " + Tr("Truk")},
		zoneCityInfo{"Antarctica/Casey", "UTC+11 " + Tr("Casey")},
		zoneCityInfo{"Asia/Sakhalin", "UTC+11 " + Tr("Sakhalin")},
		zoneCityInfo{"Asia/Vladivostok", "UTC+11 " + Tr("Vladivostok")},
		zoneCityInfo{"Australia/Lord_Howe", "UTC+11 " + Tr("Lord Howe")},
		zoneCityInfo{"Pacific/Efate", "UTC+11 " + Tr("Efate")},
		zoneCityInfo{"Pacific/Efate", "UTC+11 " + Tr("Nouméa")},
		zoneCityInfo{"Pacific/Guadalcanal", "UTC+11 " + Tr("Guadalcanal")},
		zoneCityInfo{"Pacific/Kosrae", "UTC+11 " + Tr("Kosrae")},
		zoneCityInfo{"Pacific/Norfolk", "UTC+11 " + Tr("Norfolk")},
		zoneCityInfo{"Pacific/Ponape", "UTC+11 " + Tr("Ponape")},
		zoneCityInfo{"Antarctica/McMurdo", "UTC+12 " + Tr("McMurdo")},
		zoneCityInfo{"Antarctica/South_Pole", "UTC+12 " + Tr("South Pole")},
		zoneCityInfo{"Asia/Anadyr", "UTC+12 " + Tr("Anadyr")},
		zoneCityInfo{"Asia/Kabul", "UTC+12 " + Tr("Kabul")},
		zoneCityInfo{"Asia/Magadan", "UTC+12 " + Tr("Magadan")},
		zoneCityInfo{"NZ", "UTC+12 " + Tr("Wellington")},
		zoneCityInfo{"Pacific/Auckland", "UTC+12 " + Tr("Auckland")},
		zoneCityInfo{"Pacific/Fiji", "UTC+12 " + Tr("Fiji")},
		zoneCityInfo{"Pacific/Funafuti", "UTC+12 " + Tr("Funafuti")},
		zoneCityInfo{"Pacific/Kwajalein", "UTC+12 " + Tr("Kwajalein")},
		zoneCityInfo{"Pacific/Nauru", "UTC+12 " + Tr("Nauru")},
		zoneCityInfo{"Pacific/Tarawa", "UTC+12 " + Tr("Tarawa")},
		zoneCityInfo{"Pacific/Wake", "UTC+12 " + Tr("Wake")},
		zoneCityInfo{"Pacific/Wallis", "UTC+12 " + Tr("Wallis")},
		zoneCityInfo{"Pacific/Enderbury", "UTC+13 " + Tr("Enderbury")},
		zoneCityInfo{"Pacific/Fakaofo", "UTC+13 " + Tr("Fakaofo")},
		zoneCityInfo{"Pacific/Tongatapu", "UTC+13 " + Tr("Tongatapu")},
	}
}
