package pgtype

import (
	"encoding/hex"
	"fmt"
	"io"
	"reflect"

	"github.com/jackc/pgx/pgio"
)

type Bytea struct {
	Bytes  []byte
	Status Status
}

func (dst *Bytea) ConvertFrom(src interface{}) error {
	switch value := src.(type) {
	case Bytea:
		*dst = value
	case []byte:
		if value != nil {
			*dst = Bytea{Bytes: value, Status: Present}
		} else {
			*dst = Bytea{Status: Null}
		}
	default:
		if originalSrc, ok := underlyingBytesType(src); ok {
			return dst.ConvertFrom(originalSrc)
		}
		return fmt.Errorf("cannot convert %v to Bytea", value)
	}

	return nil
}

func (src *Bytea) AssignTo(dst interface{}) error {
	switch v := dst.(type) {
	case *[]byte:
		if src.Status == Present {
			*v = src.Bytes
		} else {
			*v = nil
		}
	default:
		if v := reflect.ValueOf(dst); v.Kind() == reflect.Ptr {
			el := v.Elem()
			switch el.Kind() {
			// if dst is a pointer to pointer, strip the pointer and try again
			case reflect.Ptr:
				if src.Status == Null {
					el.Set(reflect.Zero(el.Type()))
					return nil
				}
				if el.IsNil() {
					// allocate destination
					el.Set(reflect.New(el.Type().Elem()))
				}
				return src.AssignTo(el.Interface())
			default:
				if originalDst, ok := underlyingPtrSliceType(dst); ok {
					return src.AssignTo(originalDst)
				}
			}
		}
		return fmt.Errorf("cannot decode %v into %T", src, dst)
	}

	return nil
}

// DecodeText only supports the hex format. This has been the default since
// PostgreSQL 9.0.
func (dst *Bytea) DecodeText(src []byte) error {
	if src == nil {
		*dst = Bytea{Status: Null}
		return nil
	}

	if len(src) < 2 || src[0] != '\\' || src[1] != 'x' {
		return fmt.Errorf("invalid hex format")
	}

	buf := make([]byte, (len(src)-2)/2)
	_, err := hex.Decode(buf, src[2:])
	if err != nil {
		return err
	}

	*dst = Bytea{Bytes: buf, Status: Present}
	return nil
}

func (dst *Bytea) DecodeBinary(src []byte) error {
	if src == nil {
		*dst = Bytea{Status: Null}
		return nil
	}

	*dst = Bytea{Bytes: src, Status: Present}
	return nil
}

func (src Bytea) EncodeText(w io.Writer) error {
	if done, err := encodeNotPresent(w, src.Status); done {
		return err
	}

	str := hex.EncodeToString(src.Bytes)

	_, err := pgio.WriteInt32(w, int32(len(str)+2))
	if err != nil {
		return nil
	}

	_, err = io.WriteString(w, `\x`)
	if err != nil {
		return nil
	}

	_, err = io.WriteString(w, str)
	return err
}

func (src Bytea) EncodeBinary(w io.Writer) error {
	if done, err := encodeNotPresent(w, src.Status); done {
		return err
	}

	_, err := pgio.WriteInt32(w, int32(len(src.Bytes)))
	if err != nil {
		return nil
	}

	_, err = w.Write(src.Bytes)
	return err
}