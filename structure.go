package mock

type Structure struct {
	Name       string
	NameAbbrev string
	Methods    []Func
}

type Func struct {
	Name    string
	Args    string
	RetArgs string
}
