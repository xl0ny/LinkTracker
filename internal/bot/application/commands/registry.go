package commands

func All() []Command {
	return []Command{
		Start{},
		Help{},
	}
}
