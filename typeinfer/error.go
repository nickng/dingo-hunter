package typeinfer

// Predefined errors

import "errors"

var (
	ErrEmptyStack      = errors.New("stack: empty")
	ErrNoMainPkg       = errors.New("no main package found")
	ErrNonConstChanBuf = errors.New("MakeChan creates channel with non-const buffer size")
	ErrMakeChanNonChan = errors.New("type error: MakeChan creates non-channel type channel")
	ErrUnitialisedFunc = errors.New("operation on uninitialised function (did you call prepareVisit?)")
	ErrUnknownValue    = errors.New("internal error: unknown SSA value")
	ErrInvalidJumpSucc = errors.New("internal error: wrong number of Succ for Jump (expects 1)")
	ErrInvalidIfSucc   = errors.New("internal error: wrong number of Succ for If (expects 2)")
	ErrUnimplemented   = errors.New("unimplemented")
	ErrWrongArgNum     = errors.New("wrong number of arguments")
	ErrRuntimeLen      = errors.New("length can only be determined at runtime")
	ErrInvalidVarWrite = errors.New("internal error: write to uninitialised variable")
	ErrInvalidVarRead  = errors.New("internal error: read from uninitialised variable")
	ErrIfaceIncomplete = errors.New("interface not fully implemented")
	ErrMethodNotFound  = errors.New("interface method not found")
	ErrPhiUnknownEdge  = errors.New("phi node has edge from unknown block")
)
