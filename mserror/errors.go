package mserror

type MsError struct {
	err      error
	errorFuc ErrorFuc
}

func (e *MsError) Error() string {
	return e.err.Error()
}
func Default() *MsError {
	return &MsError{}
}

func (e *MsError) Put(err error) {
	e.checkError(err)
}

func (e *MsError) checkError(err error) {
	if err != nil {
		e.err = err
		panic(e)
	}
}

type ErrorFuc func(msError *MsError)

func (e *MsError) Result(fuc ErrorFuc) {
	e.errorFuc = fuc
}

func (e *MsError) ExecResult() {
	e.errorFuc(e)
}
