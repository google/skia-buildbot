package cmd

type Command interface {
}

type New() Command {
	return command{}
}

// command implements Command.
type command struct {

}


// Confirm command implements the Command interface.
var _ Command = command{}