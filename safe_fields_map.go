package gorm

import (
	"errors"
	"fmt"
	"sync"
)

type (
	fieldsMap struct {
		aliases map[string]*StructField
		l       *sync.RWMutex
		fields  StructFields
	}
)

func (s *fieldsMap) Add(field *StructField) error {
	if s.l == nil {
		return errors.New("fieldsMap ERROR !!! NOT INITED!")
	}
	s.l.Lock()
	defer s.l.Unlock()
	_, hasGetName := s.aliases[field.StructName]
	_, hasDBName := s.aliases[field.DBName]
	if hasGetName || hasDBName {
		//replace in slice, even if we shouldn't (it's not correct to have this behavior)
		for index, existingField := range s.fields {
			if existingField.StructName == field.StructName || existingField.DBName == field.DBName {
				s.fields[index] = field
				break
			}
		}

	} else {
		s.fields.add(field)
	}
	s.aliases[field.StructName] = field
	s.aliases[field.DBName] = field
	return nil
}

func (s *fieldsMap) Get(key string) (*StructField, bool) {
	if s.l == nil {
		fmt.Errorf("fieldsMap ERROR : not inited")
		return nil, false
	}
	s.l.RLock()
	defer s.l.RUnlock()
	//If the requested key doesn't exist, we get the value type's zero value - avoiding that
	val, ok := s.aliases[key]
	return val, ok
}

func (s fieldsMap) Fields() StructFields {
	if s.l == nil {
		fmt.Errorf("fieldsMap ERROR : not inited")
		return nil
	}
	return s.fields
}

func (s fieldsMap) PrimaryFields() StructFields {
	if s.l == nil {
		fmt.Errorf("fieldsMap ERROR : not inited")
		return nil
	}
	s.l.RLock()
	defer s.l.RUnlock()
	var result StructFields
	for _, field := range s.fields {
		if field.IsPrimaryKey() {
			result.add(field)
		}
	}
	return result
}
