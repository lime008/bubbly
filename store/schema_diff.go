package store

import (
	"fmt"
	"reflect"

	"github.com/verifa/bubbly/api/core"
)

// compareSchema will take 2 schemas as arguments and return a Changelog
// elements are matched on Name, and if a Name is not matched in the 2nd schema,
// this will be treated as a deletion.
func compareSchema(s1 bubblySchema, s2 bubblySchema) (Changelog, error) {
	var changelog Changelog
	for tableName, table1 := range s1.Tables {
		if tableName != table1.Name {
			return nil, fmt.Errorf("map key and table name do not match for table %s", table1.Name)
		}
		_, ok := s2.Tables[table1.Name]
		// The key exists in both tables, and will be checked for updates
		if ok {
			// ALTER
			calculateDiff(table1, s2.Tables[table1.Name], &changelog)
		} else {
			// DELETE
			// The key does not exist in the second table and will be removed
			changelog = append(changelog, Entry{
				Action: remove,
				TableInfo: tableInfo{
					TableName:   table1.Name,
					ElementName: table1.Name,
					ElementType: tableType,
				},
				From: table1,
				To:   nil,
			})
		}
	}
	for _, table2 := range s2.Tables {
		_, ok := s1.Tables[table2.Name]
		if !ok {
			// CREATE
			// The key exist in only the second table and will be created
			changelog = append(changelog, Entry{
				Action: create,
				TableInfo: tableInfo{
					TableName:   table2.Name,
					ElementName: table2.Name,
					ElementType: tableType,
				},
				From: nil,
				To:   table2,
			})
		}
	}
	return changelog, nil
}

type DiffAction string
type Element string

var (
	update DiffAction = "update"
	remove DiffAction = "delete"
	create DiffAction = "create"

	tableType  Element = "table"
	fieldType  Element = "field"
	joinType   Element = "join"
	uniqueType Element = "unique"
)

// Changelog is a list of expectedChanges that will be applied by the migration
type Changelog []Entry

type tableInfo struct {
	TableName   string
	ElementName string
	ElementType Element
}

type Entry struct {
	Action    DiffAction
	TableInfo tableInfo
	From      interface{}
	To        interface{}
}

// calculateDiff will calculate the difference between two schemas.
// In this case, all elements will be matched on id, if 2 ids are different, they will
// be treated as separate elements. For example:
// field1: "hello world" -> field1: "lizards"
// will be views as an update on field1, but if field1 has its name changed:
// field1: "hello world" -> field2: "hello world"
// These will be treated as 2 separate entities, field1 will be seen as deleted, and field2 will be added
func calculateDiff(t1 core.Table, t2 core.Table, cl *Changelog) {
	compareFields(t1, t2, cl)
	compareJoins(t1, t2, cl)
	compareTables(t1, t2, cl)
	if t1.Unique != t2.Unique {
		e := Entry{
			Action: update,
			TableInfo: tableInfo{
				TableName:   t2.Name,
				ElementName: "name",
				ElementType: uniqueType,
			},
			From: t1.Unique,
			To:   t2.Unique,
		}
		*cl = append(*cl, e)
	}
}

func compareFields(t1, t2 core.Table, cl *Changelog) {
mainLoop:
	for _, field1 := range t1.Fields {
		found := false
	fieldLoop:
		for _, field2 := range t2.Fields {
			if reflect.DeepEqual(field1, field2) {
				break mainLoop
			}
			if field1.Name == field2.Name && !reflect.DeepEqual(field1.Type, field2.Type) {
				*cl = append(*cl, Entry{
					Action: update,
					TableInfo: tableInfo{
						TableName:   t2.Name,
						ElementName: field2.Name,
						ElementType: fieldType,
					},
					From: field1.Type,
					To:   field2.Type,
				})
				found = true
			}
			if field1.Name == field2.Name && field1.Unique != field2.Unique {
				*cl = append(*cl, Entry{
					Action: update,
					TableInfo: tableInfo{
						TableName:   t2.Name,
						ElementName: field2.Name,
						ElementType: uniqueType,
					},
					From: field1.Unique,
					To:   field2.Unique,
				})
				found = true
				break fieldLoop
			}
		}
		if !found {
			*cl = append(*cl, Entry{
				Action: remove,
				TableInfo: tableInfo{
					TableName:   t1.Name,
					ElementName: t1.Name,
					ElementType: fieldType,
				},
				From: field1,
				To:   nil,
			})
		}
	}
	for _, schema2Field := range t2.Fields {
		found := false
		for _, schema1Field := range t1.Fields {
			if schema2Field.Name == schema1Field.Name {
				found = true
			}
		}
		if !found {
			*cl = append(*cl, Entry{
				Action: create,
				TableInfo: tableInfo{
					TableName:   t2.Name,
					ElementName: schema2Field.Name,
					ElementType: fieldType,
				},
				From: nil,
				To:   schema2Field,
			})
		}
	}
}

func compareJoins(t1, t2 core.Table, cl *Changelog) {
	for _, join1 := range t1.Joins {
		found := false
	joinLoop:
		for _, join2 := range t2.Joins {
			if join1.Table == join2.Table && join1.Unique != join2.Unique {
				*cl = append(*cl, Entry{
					Action: update,
					TableInfo: tableInfo{
						TableName:   t2.Name,
						ElementName: join2.Table,
						ElementType: joinType,
					},
					From: join1.Unique,
					To:   join2.Unique,
				})
				found = true
				break joinLoop
			} else if join1.Table == join2.Table && join1.Unique == join2.Unique {
				found = true
			}
		}
		if !found {
			*cl = append(*cl, Entry{
				Action: remove,
				TableInfo: tableInfo{
					TableName:   t1.Name,
					ElementName: join1.Table,
					ElementType: joinType,
				},
				From: join1,
				To:   nil,
			})
		}
	}

	for _, join2 := range t2.Joins {
		found := false
	join2Loop:
		for _, join1 := range t1.Joins {
			if join2.Table == join1.Table {
				found = true
				break join2Loop
			}
		}
		if !found {
			*cl = append(*cl, Entry{
				Action: create,
				TableInfo: tableInfo{
					TableName:   t2.Name,
					ElementName: join2.Table,
					ElementType: joinType,
				},
				From: nil,
				To:   join2,
			})
		}
	}
}

func compareTables(parentTable1, parentTable2 core.Table, cl *Changelog) {
	for _, table1 := range parentTable1.Tables {
		found := false
		var subCl Changelog
		for _, table2 := range parentTable2.Tables {
			if table1.Name != table2.Name {
				continue
			}
			calculateDiff(table1, table2, &subCl)
			found = true
		}

		if !found {
			subCl = append(subCl, Entry{
				Action: remove,
				TableInfo: tableInfo{
					TableName:   table1.Name,
					ElementName: table1.Name,
					ElementType: tableType,
				},
				From: table1,
				To:   nil,
			})
		}
		cl.combine(&subCl)
	}
	for _, table2 := range parentTable2.Tables {
		found := false
		var subCl Changelog
		for _, table1 := range parentTable1.Tables {
			if table2.Name == table1.Name {
				found = true
				break
			}
		}
		if !found {
			// no more recursion will be performed since the rest of the nested values are already to be created
			subCl = append(subCl, Entry{
				Action: create,
				TableInfo: tableInfo{
					TableName:   table2.Name,
					ElementName: table2.Name,
					ElementType: tableType,
				},
				From: nil,
				To:   table2,
			})
		}
		cl.combine(&subCl)
	}
}

func (cl *Changelog) combine(newCl *Changelog) {
	for _, entry := range *newCl {
		*cl = append(*cl, entry)
	}
}
