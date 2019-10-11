package xftp

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/sftp"
)

// TSftp -
type TSftp struct {
	client *sftp.Client
	cwd    string
}

// func (o *TSftp) resolveDir(dir string) string {
// 	if path.IsAbs(dir) {
// 		return path.Clean(dir)
// 	}
// 	return path.Join(o.cwd, dir)
// }

// Exists -
func (o *TSftp) Exists(path string) error {
	// path = o.resolveDir(path)
	_, err := o.client.Stat(path)
	if err != nil {
		return err
	}
	return nil
}

// Delete -
func (o *TSftp) Delete(path string) error {
	// path = o.resolveDir(path)
	return o.client.Remove(path)
}

// Rename -
func (o *TSftp) Rename(from, to string) error {
	// from = o.resolveDir(from)
	// to = o.resolveDir(to)
	return o.client.Rename(from, to)
}

// Quit -
func (o *TSftp) Quit() error {
	return o.client.Close()
}

// FileSize -
func (o *TSftp) FileSize(path string) (int64, error) {
	// path = o.resolveDir(path)
	stat, err := o.client.Stat(path)
	if err != nil {
		return -1, err
	}
	return stat.Size(), nil
}

// StorFrom -
func (o *TSftp) StorFrom(path string, r io.Reader, offset uint64) error {
	// conn, err := c.cmdDataConnFrom(offset, "STOR %s", path)
	// if err != nil {
	// 	return err
	// }
	// path = o.resolveDir(path)
	f, err := o.client.OpenFile(path, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer f.Close()
	//_, err = io.Copy(f, r)
	offs, err := f.Seek(int64(offset), 0)
	if err != nil {
		return err
	}
	if offs != int64(offset) {
		return fmt.Errorf("Sftp Seek() problem (custom error). Search %v, but return %v", int64(offset), offs)
	}
	_, err = f.ReadFrom(r)
	if err != nil {
		return err
	}
	// _, _, err = c.conn.ReadResponse(StatusClosingDataConnection)
	// return err
	return nil
}

// ChangeDir -
// func (o *TSftp) ChangeDir(dir string) error {
// 	err := o.Exists(dir)
// 	if err != nil {
// 		return fmt.Errorf("%v %q", err, o.resolveDir(dir))
// 	}
// 	o.cwd = o.resolveDir(dir)
// 	return nil
// }

// CurrentDir -
// func (o *TSftp) CurrentDir() (string, error) {
// 	return o.cwd, nil
// }

// List -
func (o *TSftp) List(path string) ([]TEntry, error) {
	// path = o.resolveDir(path)
	src, err := o.client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	list := []TEntry{}
	for _, item := range src {
		entry := TEntry{Name: item.Name(), Size: item.Size(), Time: item.ModTime()}
		entry.Type = File
		if item.IsDir() {
			entry.Type = Folder

		}
		list = append(list, entry)
	}
	return list, nil
}
