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

func (st *Storage) ClearFeeds() error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		root.ForEachBucket(func(k []byte) error {
			b := root.Bucket(k)
			b.ForEach(func(k, v []byte) error {
				if err := b.Delete(k); err != nil {
					return err
				}
				return nil
			})
			return nil
		})
		return nil
	})
	return err
}
