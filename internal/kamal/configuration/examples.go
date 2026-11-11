package configuration

// Ports lib/kamal/configuration/validation.rb at Kamal 2.12.0.

import (
	"fmt"
	"sync"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration/docs"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

var (
	examplesMu sync.Mutex
	examples   = map[string]*yamlord.Value{}
)

// exampleDoc returns the parsed configuration document of a section, such as
// "builder". The documents are embedded, so a parse failure is a defect in the
// binary and panics with the section name.
func exampleDoc(section string) *yamlord.Value {
	examplesMu.Lock()
	defer examplesMu.Unlock()
	if doc, ok := examples[section]; ok {
		return doc
	}
	doc := parseExample(section)
	examples[section] = doc
	return doc
}

func parseExample(section string) *yamlord.Value {
	content, err := docs.Read(section)
	if err != nil {
		panic(err)
	}
	doc, err := yamlord.Parse(content)
	if err != nil {
		panic(fmt.Sprintf("configuration document %s.yml does not parse: %v", section, err))
	}
	return doc
}

// example returns validation_yml[section] of the section's own document, such as
// the "builder" key of builder.yml.
func example(section string) *yamlord.Value {
	return exampleDoc(section).Map().At(section)
}
