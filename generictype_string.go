// generated by stringer -type=GenericType; DO NOT EDIT

package main

import "fmt"

const _GenericType_name = "GT_NUMERICGT_STRINGGT_TIMESTAMP"

var _GenericType_index = [...]uint8{0, 10, 19, 31}

func (i GenericType) String() string {
	if i < 0 || i+1 >= GenericType(len(_GenericType_index)) {
		return fmt.Sprintf("GenericType(%d)", i)
	}
	return _GenericType_name[_GenericType_index[i]:_GenericType_index[i+1]]
}
