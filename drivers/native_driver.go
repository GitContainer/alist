package drivers

import (
	"fmt"
	"github.com/Xhofe/alist/conf"
	"github.com/Xhofe/alist/model"
	"github.com/Xhofe/alist/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Native struct{}

func (driver Native) Config() DriverConfig {
	return DriverConfig{
		Name:      "Native",
		OnlyProxy: true,
	}
}

func (driver Native) Items() []Item {
	return []Item{
		{
			Name:     "root_folder",
			Label:    "root folder path",
			Type:     TypeString,
			Required: true,
		},
		{
			Name:     "order_by",
			Label:    "order_by",
			Type:     TypeSelect,
			Values:   "name,size,updated_at",
			Required: false,
		},
		{
			Name:     "order_direction",
			Label:    "order_direction",
			Type:     TypeSelect,
			Values:   "ASC,DESC",
			Required: false,
		},
	}
}

func (driver Native) Save(account *model.Account, old *model.Account) error {
	log.Debugf("save a account: [%s]", account.Name)
	if !utils.Exists(account.RootFolder) {
		account.Status = fmt.Sprintf("[%s] not exist", account.RootFolder)
		_ = model.SaveAccount(account)
		return fmt.Errorf("[%s] not exist", account.RootFolder)
	}
	account.Status = "work"
	account.Proxy = true
	err := model.SaveAccount(account)
	if err != nil {
		return err
	}
	return nil
}

func (driver Native) File(path string, account *model.Account) (*model.File, error) {
	fullPath := filepath.Join(account.RootFolder, path)
	if !utils.Exists(fullPath) {
		return nil, ErrPathNotFound
	}
	f, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	time := f.ModTime()
	file := &model.File{
		Name:      f.Name(),
		Size:      f.Size(),
		UpdatedAt: &time,
		Driver:    driver.Config().Name,
	}
	if f.IsDir() {
		file.Type = conf.FOLDER
	} else {
		file.Type = utils.GetFileType(filepath.Ext(f.Name()))
	}
	return file, nil
}

func (driver Native) Files(path string, account *model.Account) ([]model.File, error) {
	fullPath := filepath.Join(account.RootFolder, path)
	if !utils.Exists(fullPath) {
		return nil, ErrPathNotFound
	}
	files := make([]model.File, 0)
	rawFiles, err := ioutil.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	for _, f := range rawFiles {
		if strings.HasPrefix(f.Name(), ".") {
			continue
		}
		time := f.ModTime()
		file := model.File{
			Name:      f.Name(),
			Size:      f.Size(),
			Type:      0,
			UpdatedAt: &time,
			Driver:    driver.Config().Name,
		}
		if f.IsDir() {
			file.Type = conf.FOLDER
		} else {
			file.Type = utils.GetFileType(filepath.Ext(f.Name()))
		}
		files = append(files, file)
	}
	model.SortFiles(files, account)
	return files, nil
}

func (driver Native) Link(path string, account *model.Account) (string, error) {
	fullPath := filepath.Join(account.RootFolder, path)
	s, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	if s.IsDir() {
		return "", fmt.Errorf("can't down folder")
	}
	return fullPath, nil
}

func (driver Native) Path(path string, account *model.Account) (*model.File, []model.File, error) {
	log.Debugf("native path: %s", path)
	file, err := driver.File(path, account)
	if err != nil {
		return nil, nil, err
	}
	if file.Type != conf.FOLDER {
		//file.Url, _ = driver.Link(path, account)
		return file, nil, nil
	}
	files, err := driver.Files(path, account)
	if err != nil {
		return nil, nil, err
	}
	model.SortFiles(files, account)
	return nil, files, nil
}

func (driver Native) Proxy(c *gin.Context, account *model.Account) {
	// unnecessary
}

func (driver Native) Preview(path string, account *model.Account) (interface{}, error) {
	return nil, ErrNotSupport
}

func (driver Native) MakeDir(path string, account *model.Account) error {
	fullPath := filepath.Join(account.RootFolder, path)
	err := os.MkdirAll(fullPath, 0700)
	return err
}

func (driver Native) Move(src string, dst string, account *model.Account) error {
	fullSrc := filepath.Join(account.RootFolder, src)
	fullDst := filepath.Join(account.RootFolder, dst)
	return os.Rename(fullSrc, fullDst)
}

func (driver Native) Copy(src string, dst string, account *model.Account) error {
	fullSrc := filepath.Join(account.RootFolder, src)
	fullDst := filepath.Join(account.RootFolder, dst)
	srcFile, err := driver.File(src, account)
	if err != nil {
		return err
	}
	dstFile, err := driver.File(dst, account)
	if err == nil {
		if !dstFile.IsDir() {
			return ErrNotSupport
		}
	}
	if srcFile.IsDir() {
		return driver.CopyDir(fullSrc, fullDst)
	}
	return driver.CopyFile(fullSrc, fullDst)
}

func (driver Native) Delete(path string, account *model.Account) error {
	fullPath := filepath.Join(account.RootFolder, path)
	file, err := driver.File(path, account)
	if err != nil {
		return err
	}
	if file.IsDir() {
		return os.RemoveAll(fullPath)
	}
	return os.Remove(fullPath)
}

func (driver Native) Upload(file *model.FileStream, account *model.Account) error {
	fullPath := filepath.Join(account.RootFolder, file.Path, file.Name)
	_, err := driver.File(filepath.Join(file.Path,file.Name), account)
	if err == nil {
		// TODO overwrite?
	}
	basePath := filepath.Dir(fullPath)
	if !utils.Exists(basePath) {
		err := os.MkdirAll(basePath, 0744)
		if err != nil {
			return err
		}
	}
	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	_, err = io.Copy(out, file)
	return err
}

var _ Driver = (*Native)(nil)