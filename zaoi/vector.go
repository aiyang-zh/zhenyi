package zaoi

// Vector2 is a simple 2D vector implementation of IVector.
// Vector2 是 IVector 的一个简单二维向量实现。
type Vector2 struct {
	X, Y float64
}

// GetX returns X component.
// GetX 返回 X 分量。
func (v Vector2) GetX() float64 {
	return v.X
}

// GetY returns Y component.
// GetY 返回 Y 分量。
func (v Vector2) GetY() float64 {
	return v.Y
}
