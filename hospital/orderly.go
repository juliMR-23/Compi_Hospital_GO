package hospital

type Orderly struct {
	Person
	patientsAttended []Patient
}
