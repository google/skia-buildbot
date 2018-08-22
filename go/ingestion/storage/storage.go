package storage

type IngestionStore struct {

}

type Rec struct{

}

func New() *IngestionStore {
	return &IngestionStore{}
}

func (is *IngestionStore) Add(commit, builder, name, md5 string, timeStamp int64) error {

}

func (is *IngestionStore)