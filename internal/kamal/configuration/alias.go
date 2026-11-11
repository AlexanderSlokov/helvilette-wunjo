package configuration

// Ports lib/kamal/configuration/alias.rb at Kamal 2.12.0.

// Alias is a named shortcut for a Kamal command line, such as console: app exec -i.
type Alias struct {
	Name    string
	Command string
}

func newAlias(name string, cfg *Configuration) (*Alias, error) {
	command := cfg.raw.At("aliases").Map().At(name)
	if err := validateAlias(command, exampleDoc("alias").Dig("aliases", "uname"), "aliases/"+name); err != nil {
		return nil, err
	}
	return &Alias{Name: name, Command: command.ToS()}, nil
}
