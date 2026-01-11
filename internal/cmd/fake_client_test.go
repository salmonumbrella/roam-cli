package cmd

import (
	"encoding/json"

	"github.com/salmonumbrella/roam-cli/internal/api"
)

type fakeClient struct {
	QueryFunc                  func(string, ...interface{}) ([][]interface{}, error)
	PullFunc                   func(interface{}, string) (json.RawMessage, error)
	PullManyFunc               func([]interface{}, string) (json.RawMessage, error)
	CreateBlockFunc            func(string, string, interface{}) error
	CreateBlockWithOptionsFunc func(string, api.BlockOptions, interface{}) error
	CreateBlockAtLocationFunc  func(api.Location, api.BlockOptions) error
	UpdateBlockFunc            func(string, string) error
	UpdateBlockWithOptionsFunc func(string, api.BlockOptions) error
	MoveBlockFunc              func(string, string, interface{}) error
	MoveBlockToLocationFunc    func(string, api.Location) error
	DeleteBlockFunc            func(string) error
	CreatePageFunc             func(string) error
	CreatePageWithOptionsFunc  func(api.PageOptions) error
	UpdatePageFunc             func(string, string) error
	UpdatePageWithOptionsFunc  func(string, api.PageOptions) error
	DeletePageFunc             func(string) error
	ExecuteBatchFunc           func(*api.BatchBuilder) error
	GetPageByTitleFunc         func(string) (json.RawMessage, error)
	GetBlockByUIDFunc          func(string) (json.RawMessage, error)
	SearchBlocksFunc           func(string, int) ([][]interface{}, error)
	ListPagesFunc              func(bool, int) ([][]interface{}, error)
}

func (f *fakeClient) Query(query string, args ...interface{}) ([][]interface{}, error) {
	if f.QueryFunc != nil {
		return f.QueryFunc(query, args...)
	}
	return nil, nil
}

func (f *fakeClient) Pull(eid interface{}, selector string) (json.RawMessage, error) {
	if f.PullFunc != nil {
		return f.PullFunc(eid, selector)
	}
	return nil, nil
}

func (f *fakeClient) PullMany(eids []interface{}, selector string) (json.RawMessage, error) {
	if f.PullManyFunc != nil {
		return f.PullManyFunc(eids, selector)
	}
	return nil, nil
}

func (f *fakeClient) CreateBlock(parentUID, content string, order interface{}) error {
	if f.CreateBlockFunc != nil {
		return f.CreateBlockFunc(parentUID, content, order)
	}
	return nil
}

func (f *fakeClient) CreateBlockWithOptions(parentUID string, opts api.BlockOptions, order interface{}) error {
	if f.CreateBlockWithOptionsFunc != nil {
		return f.CreateBlockWithOptionsFunc(parentUID, opts, order)
	}
	return nil
}

func (f *fakeClient) CreateBlockAtLocation(loc api.Location, opts api.BlockOptions) error {
	if f.CreateBlockAtLocationFunc != nil {
		return f.CreateBlockAtLocationFunc(loc, opts)
	}
	return nil
}

func (f *fakeClient) UpdateBlock(uid, content string) error {
	if f.UpdateBlockFunc != nil {
		return f.UpdateBlockFunc(uid, content)
	}
	return nil
}

func (f *fakeClient) UpdateBlockWithOptions(uid string, opts api.BlockOptions) error {
	if f.UpdateBlockWithOptionsFunc != nil {
		return f.UpdateBlockWithOptionsFunc(uid, opts)
	}
	return nil
}

func (f *fakeClient) MoveBlock(uid, parentUID string, order interface{}) error {
	if f.MoveBlockFunc != nil {
		return f.MoveBlockFunc(uid, parentUID, order)
	}
	return nil
}

func (f *fakeClient) MoveBlockToLocation(uid string, loc api.Location) error {
	if f.MoveBlockToLocationFunc != nil {
		return f.MoveBlockToLocationFunc(uid, loc)
	}
	return nil
}

func (f *fakeClient) DeleteBlock(uid string) error {
	if f.DeleteBlockFunc != nil {
		return f.DeleteBlockFunc(uid)
	}
	return nil
}

func (f *fakeClient) CreatePage(title string) error {
	if f.CreatePageFunc != nil {
		return f.CreatePageFunc(title)
	}
	return nil
}

func (f *fakeClient) CreatePageWithOptions(opts api.PageOptions) error {
	if f.CreatePageWithOptionsFunc != nil {
		return f.CreatePageWithOptionsFunc(opts)
	}
	return nil
}

func (f *fakeClient) UpdatePage(uid, title string) error {
	if f.UpdatePageFunc != nil {
		return f.UpdatePageFunc(uid, title)
	}
	return nil
}

func (f *fakeClient) UpdatePageWithOptions(uid string, opts api.PageOptions) error {
	if f.UpdatePageWithOptionsFunc != nil {
		return f.UpdatePageWithOptionsFunc(uid, opts)
	}
	return nil
}

func (f *fakeClient) DeletePage(uid string) error {
	if f.DeletePageFunc != nil {
		return f.DeletePageFunc(uid)
	}
	return nil
}

func (f *fakeClient) ExecuteBatch(batch *api.BatchBuilder) error {
	if f.ExecuteBatchFunc != nil {
		return f.ExecuteBatchFunc(batch)
	}
	return nil
}

func (f *fakeClient) GraphName() string {
	return "test-graph"
}

func (f *fakeClient) GetPageByTitle(title string) (json.RawMessage, error) {
	if f.GetPageByTitleFunc != nil {
		return f.GetPageByTitleFunc(title)
	}
	return nil, nil
}

func (f *fakeClient) GetBlockByUID(uid string) (json.RawMessage, error) {
	if f.GetBlockByUIDFunc != nil {
		return f.GetBlockByUIDFunc(uid)
	}
	return nil, nil
}

func (f *fakeClient) SearchBlocks(text string, limit int) ([][]interface{}, error) {
	if f.SearchBlocksFunc != nil {
		return f.SearchBlocksFunc(text, limit)
	}
	return nil, nil
}

func (f *fakeClient) ListPages(modifiedToday bool, limit int) ([][]interface{}, error) {
	if f.ListPagesFunc != nil {
		return f.ListPagesFunc(modifiedToday, limit)
	}
	return nil, nil
}
