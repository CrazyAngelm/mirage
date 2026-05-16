package modes

type Mode string

const (
	Normal Mode = "normal"
	Cheap  Mode = "cheap"
	Full   Mode = "full"
	Super  Mode = "super"
)

type Outbound struct {
	Tag       string
	Reachable bool
}

func Normalize(value string) Mode {
	switch Mode(value) {
	case Cheap:
		return Cheap
	case Full:
		return Full
	case Super:
		return Super
	case Normal:
		return Normal
	default:
		return Normal
	}
}

func IsTun(mode Mode) bool {
	return mode == Full
}

func Select(mode Mode, outbounds []Outbound) []Outbound {
	if mode != Cheap {
		return outbounds
	}
	for _, preferred := range []string{"hysteria2", "xray_reality_xhttp", "amneziawg"} {
		for _, outbound := range outbounds {
			if outbound.Tag == preferred && outbound.Reachable {
				return []Outbound{outbound}
			}
		}
	}
	if len(outbounds) == 0 {
		return nil
	}
	return []Outbound{outbounds[0]}
}
