package sqlStore

import (
	"github.com/holdno/gocommons/selection"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type commonFields struct {
	provider SqlProviderInterface
	table    string
}

func (c *commonFields) SetProvider(p SqlProviderInterface) {
	c.provider = p
}

func (c *commonFields) GetTable() string {
	return c.table
}

func (c *commonFields) SetTable(table string) {
	c.table = table
}

func (c *commonFields) GetMap(selector selection.Selector) ([]map[string]interface{}, error) {
	db := parseSelector(c.GetReplica(), selector, true)
	rows, err := db.Table(c.GetTable()).Rows()
	if err != nil {
		return nil, err
	}
	res, err := scanToMap(rows)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return res, nil
}

func (c *commonFields) GetTotal(selector selection.Selector) (int, error) {
	var (
		err   error
		total int
	)

	db := parseSelector(c.GetReplica(), selector, false)

	if err = db.Table(c.GetTable()).Count(&total).Error; err != nil {
		return total, err
	}

	return total, nil
}

func (c *commonFields) GetMaster() *gorm.DB {
	return c.provider.GetMaster()
}

func (c *commonFields) GetReplica() *gorm.DB {
	return c.provider.GetReplica()
}

func (c *commonFields) CheckSelf() {
	if c.provider == nil {
		panic("can not found provider")
	}

	if c.table == "" {
		panic("can not set table")
	}

	c.provider.Logger().Infof("store %s is ok!", c.GetTable())
}

type SqlProviderInterface interface {
	GetMaster() *gorm.DB
	GetReplica() *gorm.DB
	Logger() *logrus.Logger
}
