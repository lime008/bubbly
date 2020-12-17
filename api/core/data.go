package core

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var DataRefType = cty.Capsule("DataRef", reflect.TypeOf((*DataRef)(nil)))

// DataRef is a data block that does not contain a
// static value but references a value from another
// data block.
type DataRef struct {
	TableName string
	Field     string
}

// DataRefs is a slice of DataRef.
type DataRefs []DataRef

// DataBlocks is a slice of type Data
type DataBlocks []Data

// Data will reference a Table name, and assign the Field values into the
// corresponding Field values in the Table
type Data struct {
	TableName string `hcl:",label" json:"data"`
	// RowName   string     `hcl:",label" json:"row"`
	Fields   DataFields `hcl:"field,block" json:"fields"`
	Data     DataBlocks `hcl:"data,block" json:"nested_data"`
	DataRefs DataRefs
}

// DataFields is a slice of DataField
type DataFields []DataField

// DataField is a field within a Data block
type DataField struct {
	Name  string    `hcl:",label" json:"name"`
	Value cty.Value `hcl:"value,attr" json:"-"`
}

// MarshalJSON implements a JSON marshaller for Field
func (f *DataField) MarshalJSON() ([]byte, error) {
	return json.Marshal(NewJSONField(f))
}

// UnmarshalJSON implements a JSON unmarshaller for Field
func (f *DataField) UnmarshalJSON(data []byte) error {
	var jf JSONDataField
	if err := json.Unmarshal(data, &jf); err != nil {
		return fmt.Errorf("failed to unmarshal Field: %w", err)
	}
	*f = jf.Field()
	return nil
}

// DataFieldAlias is an alias to avoid a recursive stack overflow with JSONField
type DataFieldAlias DataField

// JSONDataField is a JSON-friendly version of Field
type JSONDataField struct {
	DataFieldAlias
	Value ctyjson.SimpleJSONValue `json:"value"`
}

// Field returns a Field equivalent of JSONField
func (f *JSONDataField) Field() DataField {
	field := DataField(f.DataFieldAlias)
	field.Value = f.Value.Value
	return field
}

// NewJSONField creates a new JSONField based on the given Field
func NewJSONField(f *DataField) *JSONDataField {
	return &JSONDataField{
		DataFieldAlias: DataFieldAlias(*f),
		Value: ctyjson.SimpleJSONValue{
			Value: f.Value,
		},
	}
}
