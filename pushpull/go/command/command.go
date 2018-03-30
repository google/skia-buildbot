package command

type Action string

// Action constants.
const (
	Start   Action = "start"
	Stop    Action = "stop"
	Restart Action = "restart"
	Pull    Action = "pull"
)

type Command struct {
	Service string // Not used for the Pull action.
	Action  Action
}
