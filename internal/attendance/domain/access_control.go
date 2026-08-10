package domain

type AccessMethod int32

const (
	AccessMethodUnknown  AccessMethod = 0
	AccessMethodCard     AccessMethod = 1
	AccessMethodPassword AccessMethod = 2
	AccessMethodFace     AccessMethod = 3
	AccessMethodButton   AccessMethod = 5
	AccessMethodFaceOpen AccessMethod = 15
)

func (m AccessMethod) String() string {
	switch m {
	case AccessMethodCard:
		return "card"
	case AccessMethodPassword:
		return "password"
	case AccessMethodFace:
		return "face"
	case AccessMethodButton:
		return "button"
	case AccessMethodFaceOpen:
		return "face_open"
	default:
		return "unknown"
	}
}
