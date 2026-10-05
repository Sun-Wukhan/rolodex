package mockvendor

// Record is one person known to a mock vendor.
type Record struct {
	Name    string
	Phone   string
	Street  string
	City    string
	Region  string
	Postal  string
	Country string
}

// ABCRecords is ABC's dataset. Alan Turing is deliberately absent (not_found),
// and Katherine Johnson has a stale street address (shows an unverified field).
func ABCRecords() []Record {
	return []Record{
		{"Ada Lovelace", "+1 416 555 0101", "100 King St W", "Toronto", "ON", "M5X 1A9", "CA"},
		{"Grace Hopper", "212-555-0102", "350 Fifth Ave", "New York", "NY", "10118", "US"},
		{"Katherine Johnson", "(757) 555-0104", "12 Old Mill Rd", "Hampton", "VA", "23666", "US"},
	}
}

// XYCRecords is XYC's dataset, returned in XYC's own response shape.
func XYCRecords() []Record {
	return []Record{
		{"Ada Lovelace", "4165550101", "100 King St W", "Toronto", "ON", "M5X1A9", "ca"},
		{"Alan Turing", "+44 20 7946 0103", "1 Bletchley Park", "Milton Keynes", "Buckinghamshire", "MK3 6EB", "gb"},
		{"Katherine Johnson", "7575550104", "1 NASA Dr", "Hampton", "VA", "23681", "us"},
	}
}
