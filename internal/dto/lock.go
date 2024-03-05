package dto

type Lock struct {
	Name      string
	ClientID  string
	Version   uint64
	UpdatedAt int64
}
