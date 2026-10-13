package catalog

// Address is the postal address of a customer.
//
//+reference:builder defaults=defaultAddress
type Address struct {
	// Street is the street and the house number.
	//
	//+reference:default "Dam 1"
	Street string `default:"Main 1"`
	// City is the city.
	//
	//+reference:default Amsterdam
	//+reference:sample value=Utrecht alternate=Leiden
	City string
	// Country is the ISO 3166-1 code of the country.
	//
	//+reference:sample alternate=DE
	Country string `default:"NL"`
	// Floor is the floor of the address, and zero for the ground floor.
	Floor int `default:"0"`
	// Lines are the extra lines of the address.
	Lines []string
	// Labels are the labels of the address.
	Labels map[string]struct{}
	// Notes are the notes of the address by their author.
	Notes map[string]string
	// Raw is the address as the customer typed it.
	Raw []byte
	// Unit is the unit of the address in a building, and nil for none.
	Unit *string
}

// defaultAddress returns the address that a builder starts from.
func defaultAddress() Address {
	return Address{Country: "BE"}
}
