package utils

import (
    "encoding/json"
    "fmt"
    "log"
    "reflect"
    "time"
    
    "go.etcd.io/bbolt"
)

// CacheItem 支持多种类型的缓存项
type CacheItem struct {
    DataType   string
    DataValue  interface{}
    Expiration int64
}

type Cache struct {
    db         *bbolt.DB
    bucketName []byte
}

func NewCache(dbPath, bucketName string) (*Cache, error) {
    db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
    if err != nil {
        return nil, fmt.Errorf("打开数据库失败: %v", err)
    }
    
    err = db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte(bucketName))
        return err
    })
    
    if err != nil {
        return nil, fmt.Errorf("创建存储桶失败: %v", err)
    }
    
    return &Cache{
        db:         db,
        bucketName: []byte(bucketName),
    }, nil
}

func (c *Cache) Set(key string, value interface{}, expiration int64) error {
    t := reflect.TypeOf(value)
    v := reflect.ValueOf(value)
    
    if v.Kind() == reflect.Ptr && v.IsNil() {
        return fmt.Errorf("不支持存储 nil 指针")
    }
    
    if expiration != 0 {
        expiration = time.Now().Unix() + expiration
    }
    item := CacheItem{
        DataType:   t.String(),
        DataValue:  value,
        Expiration: expiration,
    }
    
    itemData, err := json.Marshal(item)
    if err != nil {
        return fmt.Errorf("序列化失败: %v", err)
    }
    
    return c.db.Update(func(tx *bbolt.Tx) error {
        return tx.Bucket(c.bucketName).Put([]byte(key), itemData)
    })
}

func (c *Cache) Get(key string, result interface{}) error {
    err := c.db.Update(func(tx *bbolt.Tx) error {
        bucket := tx.Bucket(c.bucketName)
        data := bucket.Get([]byte(key))
        if data == nil {
            return nil
        }
        
        var item CacheItem
        if err := json.Unmarshal(data, &item); err != nil {
            return fmt.Errorf("反序列化失败: %v", err)
        }
        
        if item.Expiration > 0 && time.Now().Unix() > item.Expiration {
            return bucket.Delete([]byte(key))
        }
        
        resultData, err := json.Marshal(item.DataValue)
        if err != nil {
            return fmt.Errorf("二次序列化失败: %v", err)
        }
        
        if err = json.Unmarshal(resultData, result); err != nil {
            return fmt.Errorf("类型转换失败: %v", err)
        }
        
        return nil
    })
    return err
}

func (c *Cache) Delete(key string) error {
    return c.db.Update(func(tx *bbolt.Tx) error {
        return tx.Bucket(c.bucketName).Delete([]byte(key))
    })
}

func (c *Cache) Close() error {
    return c.db.Close()
}

func (c *Cache) Flush() error {
    return c.db.Update(func(tx *bbolt.Tx) error {
        if err := tx.DeleteBucket(c.bucketName); err != nil {
            return err
        }
        _, err := tx.CreateBucket(c.bucketName)
        return err
    })
}

func GetCache() *Cache {
    cache, err := NewCache("cache.db", "support-workflow")
    if err != nil {
        log.Fatalf("Init cache failed: %v", err)
    }
    return cache
}
