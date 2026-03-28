package zaoi

import (
	"math"
	"sync"

	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
)

// InvalidGridID is sentinel value for invalid grid ID.
// InvalidGridID 无效格子ID哨兵值。
const InvalidGridID = math.MaxUint32

// EntityNode is a doubly-linked node reused from pool to reduce GC pressure.
// EntityNode 双向链表节点（池化复用，避免高频 Add/Remove 产生 GC 压力）。
type EntityNode struct {
	Entity IEntity
	Prev   *EntityNode
	Next   *EntityNode
	GridID uint32
}

var (
	nodePoolOnce sync.Once
	nodePool     *zpool.Pool[*EntityNode]
)

func initNodePool() {
	nodePoolOnce.Do(func() {
		nodePool = zpoolobs.NewObservedPool(zpoolobs.PoolNameZAoiEntityNode, func() *EntityNode {
			return &EntityNode{GridID: InvalidGridID}
		})
	})
}

func getNode() *EntityNode {
	initNodePool()
	return nodePool.Get()
}

func putNode(node *EntityNode) {
	node.Entity = nil
	node.Prev = nil
	node.Next = nil
	node.GridID = InvalidGridID
	initNodePool()
	nodePool.Put(node)
}

// Grid is a runtime-only grid cell; bounds are computed on demand by StaticAoi.
// Grid 网格单元（瘦身版：仅保留运行时状态，边界由 StaticAoi 按需计算）。
type Grid struct {
	head  *EntityNode // 链表头
	id    uint32      // 网格唯一ID = y*gridX + x
	count int
}

// GetId returns grid unique ID.
// GetId 返回格子唯一 ID。
func (g *Grid) GetId() uint32 { return g.id }

// GetHead returns the head node of entity linked list in this grid.
// GetHead 返回该格子内实体链表头节点。
func (g *Grid) GetHead() *EntityNode { return g.head }

// Count returns entity count in this grid.
// Count 返回该格子内实体数量。
func (g *Grid) Count() int { return g.count }

// Add inserts entity into grid (node from pool, allocation-free path).
// Add 将实体加入格子（从对象池获取节点，零分配）。
func (g *Grid) Add(entity IEntity) {
	node := getNode()
	node.Entity = entity
	node.GridID = g.id
	node.Next = g.head
	node.Prev = nil

	if g.head != nil {
		g.head.Prev = node
	}
	g.head = node
	entity.SetAoiNode(node)
	g.count++
}

// Remove removes entity from grid and returns node to pool.
// Remove 将实体从格子移除（节点归还池）。调用方必须保证 entity 当前就在本格
// （即 entity.GetAoiNode().GridID == g.id），否则不执行移除并直接 return。
func (g *Grid) Remove(entity IEntity) {
	node := entity.GetAoiNode()
	if node == nil || node.GridID != g.id {
		return
	}

	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		g.head = node.Next
	}

	if node.Next != nil {
		node.Next.Prev = node.Prev
	}

	entity.SetAoiNode(nil)
	g.count--
	putNode(node)
}
