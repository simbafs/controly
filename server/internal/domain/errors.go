package domain

// Error Codes
const (
	// Connection Errors (1xxx)
	ErrInvalidQueryParams = 1001
	ErrInvalidClientType  = 1002

	// Display Registration Errors (2xxx)
	ErrCommandURLUnreachable = 2001
	ErrInvalidCommandJSON    = 2002
	ErrDisplayIDConflict     = 2003

	// Controller Connection Errors (3xxx)
	ErrTargetDisplayNotFound          = 3001
	ErrTargetDisplayAlreadyControlled = 3002
	ErrControllerIDConflict           = 3003
	ErrNotSubscribedToDisplay         = 3004

	// Communication Errors (4xxx)
	ErrInvalidMessageFormat = 4001
	ErrUnknownCommand       = 4002
	ErrInvalidCommandArgs   = 4003
	ErrInvalidCommandFormat = 4004 // Added for specific error in main.go
)
