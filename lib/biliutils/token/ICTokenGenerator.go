package token

import "time"

// ICTokenGenerator defines the token generation strategy used by Bilibili
// ticket ordering.
type ICTokenGenerator interface {
	GenerateTokenPrepareStage() string
	GenerateTokenCreateStage(whenGenPToken time.Time) string
	IsHotProject() bool
}
