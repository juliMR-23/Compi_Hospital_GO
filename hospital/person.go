package hospital

type Person struct {
	id   string
	name string
	age  int
}

func NewPerson(id string, name string, age int) Person {
	return Person{
		id:   id,
		name: name,
		age:  age,
	}
}

func (p Person) ID() string   { return p.id }
func (p Person) Name() string { return p.name }
func (p Person) Age() int     { return p.age }
