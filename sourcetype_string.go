// generated by stringer -type=SourceType; DO NOT EDIT

package main

import "fmt"

const _SourceType_name = "ST_UNKNOWNST_INT64ST_INTST_STRINGST_TIME"

var _SourceType_index = [...]uint8{0, 10, 18, 24, 33, 40}

func (i SourceType) String() string {
	if i < 0 || i+1 >= SourceType(len(_SourceType_index)) {
		return fmt.Sprintf("SourceType(%d)", i)
	}
	return _SourceType_name[_SourceType_index[i]:_SourceType_index[i+1]]
}
