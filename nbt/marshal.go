package nbt

import (
	"errors"
	"io"
	"math"
	"reflect"
)

func Marshal(w io.Writer, v interface{}) error {
	return NewEncoder(w).Encode(v)
}

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(v interface{}) error {
	val := reflect.ValueOf(v)
	return e.marshal(val, "")
}

func (e *Encoder) marshal(val reflect.Value, tagName string) error {
	switch vk := val.Kind(); vk {
	default:
		return errors.New("unknown type " + vk.String() + " whilst serializing " + tagName)

	case reflect.Uint8:
		if err := e.writeTag(TagByte, tagName); err != nil {
			return err
		}
		_, err := e.w.Write([]byte{byte(val.Uint())})
		return err

	case reflect.Int16, reflect.Uint16:
		if err := e.writeTag(TagShort, tagName); err != nil {
			return err
		}
		return e.writeInt16(int16(val.Int()))

	case reflect.Int32, reflect.Uint32:
		if err := e.writeTag(TagInt, tagName); err != nil {
			return err
		}
		return e.writeInt32(int32(val.Int()))

	case reflect.Float32:
		if err := e.writeTag(TagFloat, tagName); err != nil {
			return err
		}
		return e.writeInt32(int32(math.Float32bits(float32(val.Float()))))

	case reflect.Int64, reflect.Uint64:
		if err := e.writeTag(TagLong, tagName); err != nil {
			return err
		}
		return e.writeInt64(int64(val.Int()))

	case reflect.Float64:
		if err := e.writeTag(TagDouble, tagName); err != nil {
			return err
		}
		return e.writeInt64(int64(math.Float64bits(val.Float())))

	case reflect.Array, reflect.Slice:
		elementKind := val.Type().Elem().Kind()
		err, done := e.marshalArray(val, tagName, elementKind)
		if done {
			return err
		}

	case reflect.String:
		if err := e.writeTag(TagString, tagName); err != nil {
			return err
		}
		if err := e.writeInt16(int16(val.Len())); err != nil {
			return err
		}
		_, err := e.w.Write([]byte(val.String()))
		return err

	case reflect.Struct:
		if err := e.writeTag(TagCompound, tagName); err != nil {
			return err
		}

		return e.marshalStruct(val)

	case reflect.Map:
		if val.Type().Key().Kind() != reflect.String {
			return errors.New("unknown key type " + val.Type().String() + " for map")
		}
		if err := e.writeTag(TagCompound, tagName); err != nil {
			return err
		}

		return e.marshalMap(val)

	case reflect.Interface:
		return e.marshal(val.Elem(), tagName)
	}

	return nil
}

func (e *Encoder) marshalArray(val reflect.Value, tagName string, elementKind reflect.Kind) (error, bool) {
	switch elementKind {
	case reflect.Uint8: // []byte
		if err := e.writeTag(TagByteArray, tagName); err != nil {
			return err, true
		}
		if err := e.writeInt32(int32(val.Len())); err != nil {
			return err, true
		}
		_, err := e.w.Write(val.Bytes())
		return err, true

	case reflect.Int32:
		if err := e.writeTag(TagIntArray, tagName); err != nil {
			return err, true
		}
		n := val.Len()
		if err := e.writeInt32(int32(n)); err != nil {
			return err, true
		}
		for i := 0; i < n; i++ {
			if err := e.writeInt32(int32(val.Index(i).Int())); err != nil {
				return err, true
			}
		}

	case reflect.Int64:
		if err := e.writeTag(TagLongArray, tagName); err != nil {
			return err, true
		}
		n := val.Len()
		if err := e.writeInt32(int32(n)); err != nil {
			return err, true
		}
		for i := 0; i < n; i++ {
			if err := e.writeInt64(val.Index(i).Int()); err != nil {
				return err, true
			}
		}

	case reflect.Struct, reflect.Map: // Compounds
		if err := e.writeTag(TagList, tagName); err != nil {
			return err, true
		}

		if err := e.writeNamelessTag(TagCompound, tagName); err != nil {
			return err, true
		}

		n := val.Len()
		if err := e.writeInt32(int32(n)); err != nil {
			return err, true
		}
		if elementKind == reflect.Struct {
			for i := 0; i < n; i++ {
				if err := e.marshalStruct(val.Index(i)); err != nil {
					return err, true
				}
			}
		} else {
			for i := 0; i < n; i++ {
				if err := e.marshalMap(val.Index(i)); err != nil {
					return err, true
				}
			}
		}

	case reflect.Float32:
		if err := e.writeTag(TagList, tagName); err != nil {
			return err, true
		}
		if err := e.writeNamelessTag(TagFloat, tagName); err != nil {
			return err, true
		}
		n := val.Len()
		if err := e.writeInt32(int32(n)); err != nil {
			return err, true
		}
		for i := 0; i < n; i++ {
			if err := e.writeInt32(int32(math.Float32bits(float32(val.Index(i).Float())))); err != nil {
				return err, true
			}
		}

	case reflect.Float64:
		if err := e.writeTag(TagList, tagName); err != nil {
			return err, true
		}
		if err := e.writeNamelessTag(TagDouble, tagName); err != nil {
			return err, true
		}
		n := val.Len()
		if err := e.writeInt32(int32(n)); err != nil {
			return err, true
		}
		for i := 0; i < n; i++ {
			if err := e.writeInt64(int64(math.Float64bits(val.Index(i).Float()))); err != nil {
				return err, true
			}
		}

	case reflect.String:
		if err := e.writeTag(TagList, tagName); err != nil {
			return err, true
		}
		if err := e.writeNamelessTag(TagString, tagName); err != nil {
			return err, true
		}
		n := val.Len()
		if err := e.writeInt32(int32(n)); err != nil {
			return err, true
		}
		for i := 0; i < n; i++ {
			entry := val.Index(i)
			if err := e.writeInt16(int16(entry.Len())); err != nil {
				return err, true
			}
			if _, err := e.w.Write([]byte(entry.String())); err != nil {
				return err, true
			}
		}

	case reflect.Interface:
		// Ensure they have the same kind, then retry with the correct kind
		var realSliceType reflect.Type
		n := val.Len()
		for i := 0; i < n; i++ {
			if realSliceType == nil {
				realSliceType = val.Index(i).Elem().Type()
			} else if realSliceType != val.Index(i).Elem().Type() {
				return errors.New("mixed types in slice: found " + val.Index(i).Type().String() + " and " +
					realSliceType.String()), true
			}
		}

		if realSliceType == nil {
			if err := e.writeTag(TagList, tagName); err != nil {
				return err, true
			}
			if err := e.writeNamelessTag(TagEnd, tagName); err != nil {
				return err, true
			}
			if err := e.writeInt32(0); err != nil {
				return err, true
			}
			return nil, true
		} else {
			if realSliceType.Kind() == reflect.Interface {
				return errors.New("true slice type is interface{}, resulting in infinite recursion"), true
			} else {
				// Found the true slice type. Create an array with the proper type and use that.
				fixedArray := reflect.MakeSlice(reflect.SliceOf(realSliceType), n, n)
				for i := 0; i < n; i++ {
					fixedArray.Index(i).Set(val.Index(i).Elem())
				}
				return e.marshalArray(fixedArray, tagName, realSliceType.Kind())
			}
		}

	default:
		return errors.New("unknown type " + val.Type().String() + " slice"), true
	}
	return nil, false
}

func (e *Encoder) marshalStruct(val reflect.Value) error {
	n := val.NumField()
	for i := 0; i < n; i++ {
		f := val.Type().Field(i)
		tag := f.Tag.Get("nbt")
		if (f.PkgPath != "" && !f.Anonymous) || tag == "-" {
			continue // Private field
		}

		tagName := f.Name
		if tag != "" {
			tagName = tag
		}

		err := e.marshal(val.Field(i), tagName)
		if err != nil {
			return err
		}
	}
	_, err := e.w.Write([]byte{TagEnd})
	return err
}

func (e *Encoder) marshalMap(val reflect.Value) error {
	iter := val.MapRange()
	for iter.Next() {
		err := e.marshal(iter.Value(), iter.Key().String())
		if err != nil {
			return err
		}
	}
	_, err := e.w.Write([]byte{TagEnd})
	return err
}

func (e *Encoder) writeTag(tagType byte, tagName string) error {
	if _, err := e.w.Write([]byte{tagType}); err != nil {
		return err
	}
	bName := []byte(tagName)
	if err := e.writeInt16(int16(len(bName))); err != nil {
		return err
	}
	_, err := e.w.Write(bName)
	return err
}

func (e *Encoder) writeNamelessTag(tagType byte, tagName string) error {
	_, err := e.w.Write([]byte{tagType})
	return err
}

func (e *Encoder) writeInt16(n int16) error {
	_, err := e.w.Write([]byte{byte(n >> 8), byte(n)})
	return err
}

func (e *Encoder) writeInt32(n int32) error {
	_, err := e.w.Write([]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	return err
}

func (e *Encoder) writeInt64(n int64) error {
	_, err := e.w.Write([]byte{
		byte(n >> 56), byte(n >> 48), byte(n >> 40), byte(n >> 32),
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	return err
}
