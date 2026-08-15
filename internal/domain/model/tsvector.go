package model

import (
	"database/sql/driver"
	"fmt"
)

// TSVector handles PostgreSQL tsvector columns cleanly
type TSVector string

// Scan implements the sql.Scanner interface
func (ts *TSVector) Scan(value interface{}) error {
	if value == nil {
		*ts = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*ts = TSVector(v)
	case []byte:
		*ts = TSVector(string(v))
	default:
		return fmt.Errorf("cannot scan %T into TSVector", value)
	}
	return nil
}

// Value implements the driver.Valuer interface
func (ts TSVector) Value() (driver.Value, error) {
	return string(ts), nil
}
