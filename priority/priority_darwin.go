package priority

const (
	Idle        = 64
	BelowNormal = 16384
	Normal      = 32
	AboveNormal = 32768
	High        = 128
	RealTime    = 256
)

func PriorityString(priority int) string {
	rc := "???????"
	return rc
}

func GetPriority() (priority int, err error) {
	return -1, nil
}

func SetPriority(priority int) error {
	return nil
}
