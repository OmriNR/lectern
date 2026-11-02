package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const MaxUploadSize = 5 * 1024 * 1024

func CheckUploadFile(path string) (int64, error) {
	info, err := os.Stat(path)

	if err != nil {
		return 0, err
	}

	if info.IsDir() {
		return 0, errors.New("that's a folder, pick a file")
	}

	if info.Size() > MaxUploadSize {
		return info.Size(), fmt.Errorf("%s is %s, the limit is %s",
			filepath.Base(path),
			HumanSize(info.Size()), HumanSize(MaxUploadSize))
	}

	return info.Size(), nil
}
