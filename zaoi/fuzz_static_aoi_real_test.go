package zaoi

import (
	"testing"
)

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func FuzzStaticAoi_AddUpdateNearby_NoPanic(f *testing.F) {
	// Keep bounds small to avoid huge grids / heavy allocations.
	bounds := [4]float64{0, 800, 0, 500}
	gridSize := 50.0

	f.Add(int64(1), float64(10), float64(10), float64(40), float64(20), float64(20))
	f.Add(int64(2), float64(400), float64(250), float64(120), float64(410), float64(260))
	f.Add(int64(3), float64(-10), float64(510), float64(0), float64(10), float64(510))

	f.Fuzz(func(t *testing.T, id int64, x float64, y float64, viewDist float64, nx float64, ny float64) {
		aoi := NewStaticAoi(bounds, gridSize)

		// Clamp to a range that will sometimes be out-of-bounds to exercise ErrOutOfBounds paths.
		x = clampFloat(x, -100, bounds[1]+100)
		y = clampFloat(y, -100, bounds[3]+100)
		nx = clampFloat(nx, -100, bounds[1]+100)
		ny = clampFloat(ny, -100, bounds[3]+100)
		viewDist = clampFloat(viewDist, 0, 400)

		e := newMock(id, x, y, viewDist)
		_, _ = aoi, e

		// AddEntity may return ErrOutOfBounds; both outcomes are acceptable as long as no panic occurs.
		_ = aoi.AddEntity(e)

		// UpdateEntity may return ErrOutOfBounds; both outcomes are acceptable.
		_ = aoi.UpdateEntity(e, Vector2{X: nx, Y: ny})

		// Nearby query must not panic.
		var buf []IEntity
		aoi.GetNearbyEntities(e, &buf)

		// Remove must not panic even if Add failed (node may be nil).
		aoi.RemoveEntity(e)
	})
}
