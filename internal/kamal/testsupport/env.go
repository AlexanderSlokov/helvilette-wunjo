package testsupport

// FakeEnv is an environment for tests. Its Lookup method satisfies erb.LookupFunc and
// any other lookup that takes a variable name, so tests never read the process
// environment.
//
//	env := testsupport.FakeEnv{"DB_USER": "app"}
//	value, ok := env.Lookup("DB_USER")
type FakeEnv map[string]string

// Lookup returns the value of name and whether it is set.
func (e FakeEnv) Lookup(name string) (string, bool) {
	value, ok := e[name]
	return value, ok
}
