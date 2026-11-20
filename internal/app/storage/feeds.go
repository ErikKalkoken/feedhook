package storage

import bolt "go.etcd.io/bbolt"

func (st *Storage) ListFeeds() ([]string, error) {
	var nn []string
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		root.ForEachBucket(func(k []byte) error {
			nn = append(nn, string(k))
			return nil
		})
		return nil
	})
	return nn, err
}
