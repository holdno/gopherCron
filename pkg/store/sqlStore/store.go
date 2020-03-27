package sqlStore

import (
	"database/sql"

	"github.com/holdno/gocommons/selection"

	"github.com/jinzhu/gorm"
)

func parseSelector(db *gorm.DB, selector selection.Selector, toSelecter bool) *gorm.DB {
	for selector.NextQuery() {
		db = db.Where(selector.Patch())
	}

	if toSelecter {
		if selector.Page != 0 {
			db = db.Offset((selector.Page - 1) * selector.Pagesize)
		}

		if selector.Pagesize != 0 {
			db = db.Limit(selector.Pagesize)
		}

		if selector.Select != "" {
			db = db.Select(selector.Select)
		}

		if selector.OrderBy != "" {
			db = db.Order(selector.OrderBy)
		}
	}

	return db
}

func scanToMap(rows *sql.Rows) ([]map[string]interface{}, error) {
	var (
		cols, _ = rows.Columns()
		res     []map[string]interface{}
	)

	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))

		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make(map[string]interface{})

		for i, col := range cols {
			val := *columnPointers[i].(*interface{})

			switch val.(type) {
			case []uint8:
				m[col] = string(val.([]uint8))
			default:
				m[col] = val
			}
		}

		res = append(res, m)
	}

	if len(res) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return res, nil
}
