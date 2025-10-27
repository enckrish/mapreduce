package library

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/enckrish/remexec/client"
)

func SubmitTask(addr string, params *TaskParams) (int, error) {
	// Build CLI-like args from TaskParams
	args := params.ArgSerialize()

	it, err := client.DialAndSend(addr, ExecutablePath, args)
	if err != nil {
		return 1, fmt.Errorf("dial and send: %w", err)
	}
	defer func() { _ = it.Close() }()

	for {
		f, ok := it.Next()
		if !ok {
			if err := it.Err(); err != nil {
				return 2, fmt.Errorf("iterator error: %w", err)
			}
			break
		}
		switch f.Type {
		case client.TypeStdout:
			_, _ = os.Stdout.Write(f.Data)
		case client.TypeStderr:
			_, _ = os.Stderr.Write(f.Data)
		case client.TypeExit:
			if len(f.Data) >= 4 {
				code := int(binary.BigEndian.Uint32(f.Data[:4]))
				if code != 0 {
					return code, fmt.Errorf("remote exit code: %d", code)
				}
				return 0, nil
			}
			return 3, fmt.Errorf("invalid exit payload")
		default:
			return 4, fmt.Errorf("unknown frame type: %d", f.Type)
		}
	}
	return 0, nil
}
