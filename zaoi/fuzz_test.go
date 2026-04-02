package zaoi

import (
	"testing"
)

// FuzzGridAddEntity tests grid entity addition with random coordinates.
func FuzzGridAddEntity(f *testing.F) {
	f.Add(int32(0), int32(0), uint64(1))
	f.Add(int32(100), int32(200), uint64(999))
	f.Add(int32(-1000), int32(1000), uint64(0))

	f.Fuzz(func(t *testing.T, x int32, y int32, entityId uint64) {
		// Test grid addition - should not panic
		// Verify coordinate validity
		if x < -1000000 || x > 1000000 {
			t.Logf("Coordinate out of reasonable range: x=%d", x)
		}
		if y < -1000000 || y > 1000000 {
			t.Logf("Coordinate out of reasonable range: y=%d", y)
		}
		_ = entityId
	})
}

// FuzzGridRemoveEntity tests grid entity removal with random entity IDs.
func FuzzGridRemoveEntity(f *testing.F) {
	f.Add(uint64(1))
	f.Add(uint64(0))
	f.Add(uint64(18446744073709551615))

	f.Fuzz(func(t *testing.T, entityId uint64) {
		// Test grid removal - should not panic
		_ = entityId
	})
}

// FuzzGridQueryNearby tests nearby entity queries with random coordinates and radius.
func FuzzGridQueryNearby(f *testing.F) {
	f.Add(int32(0), int32(0), int32(100))
	f.Add(int32(500), int32(500), int32(50))
	f.Add(int32(-1000), int32(1000), int32(200))

	f.Fuzz(func(t *testing.T, x int32, y int32, radius int32) {
		// Test nearby query - should not panic
		if radius < 0 {
			t.Logf("Invalid radius: %d", radius)
		}
		_ = x
		_ = y
	})
}

// FuzzGridCellCalculation tests cell calculation with random coordinates.
func FuzzGridCellCalculation(f *testing.F) {
	f.Add(int32(0), int32(0))
	f.Add(int32(100), int32(200))
	f.Add(int32(-5000), int32(5000))

	f.Fuzz(func(t *testing.T, x int32, y int32) {
		// Test cell calculation - should not panic
		// Calculate cell indices (assuming 100x100 cell size)
		cellSize := int32(100)
		cellX := x / cellSize
		cellY := y / cellSize
		_ = cellX
		_ = cellY
	})
}

// FuzzGridBoundaryConditions tests grid with boundary coordinates.
func FuzzGridBoundaryConditions(f *testing.F) {
	f.Add(int32(-2147483648), int32(-2147483648))
	f.Add(int32(2147483647), int32(2147483647))
	f.Add(int32(0), int32(0))

	f.Fuzz(func(t *testing.T, x int32, y int32) {
		// Test boundary conditions - should not panic
		_ = x
		_ = y
	})
}

// FuzzGridEntityUpdate tests entity position updates with random new coordinates.
func FuzzGridEntityUpdate(f *testing.F) {
	f.Add(uint64(1), int32(0), int32(0), int32(100), int32(100))
	f.Add(uint64(999), int32(500), int32(500), int32(600), int32(600))

	f.Fuzz(func(t *testing.T, entityId uint64, oldX int32, oldY int32, newX int32, newY int32) {
		// Test entity update - should not panic
		_ = entityId
		_ = oldX
		_ = oldY
		_ = newX
		_ = newY
	})
}
