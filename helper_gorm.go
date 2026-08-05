package gocrud

import (
	"fmt"

	"gorm.io/gorm"
)

func GetDatabaseFieldNameOf[T any](db *gorm.DB, fields ...string) ([]string, error) {
	stmt := &gorm.Statement{DB: db}
	err := stmt.Parse(new(T))
	if err != nil {
		return nil, err
	}

	dbFields := make([]string, len(fields))
	for i, field := range fields {
		dbField := stmt.Schema.LookUpField(field)
		if dbField == nil {
			return nil, fmt.Errorf("field %s not found", field)
		}
		dbFields[i] = dbField.DBName
	}

	return dbFields, nil
}
