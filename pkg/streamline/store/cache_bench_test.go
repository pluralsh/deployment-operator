package store_test

// This benchmark test compares the performance of in-memory and file-based caches
// for the ComponentCache implementation. It measures the performance of various
// operations including initialization, setting components, retrieving children,
// deleting components, and a combined workload of setting and retrieving.
//
// Results show that the in-memory cache is generally faster than the file-based cache,
// especially for write operations like SetComponent. Here's a summary of the performance comparison:
//
// 1. SetComponent: In-memory is ~60x faster than file-based
// 2. ComponentChildren: In-memory is ~1.1x faster than file-based
// 3. DeleteComponent: In-memory is ~1.6x faster than file-based
// 4. SetComponentAndComponentChildren AndChildren: In-memory is ~3.4x faster than file-based
//
// The performance difference is most significant for write operations (SetComponent)
// because file-based operations involve disk I/O, which is much slower than memory access.
// Read operations (ComponentChildren) show a smaller performance gap because SQLite's query
// optimization works well for both storage modes.

//const (
//	benchDBFile = "/tmp/component-cache-bench.db"
//)
//
//// setupTestData creates a hierarchy of components for benchmarking
//func setupTestData(b *testing.B, cache *db.ComponentCache) {
//	state := client.ComponentState("Healthy")
//	group := testGroup
//	namespace := testNamespace
//
//	// Create root component
//	rootUID := "root-uid"
//	rootComponent := client.ComponentChildAttributes{
//		UID:       rootUID,
//		Group:     &group,
//		Version:   "v1",
//		Kind:      "Test",
//		Namespace: &namespace,
//		Name:      "root-component",
//		State:     &state,
//	}
//
//	err := cache.SetComponent(rootComponent)
//	require.NoError(b, err)
//
//	// Create 10 first-level children
//	for i := 0; i < 10; i++ {
//		uid := "level1-uid-" + string(rune('a'+i))
//		component := client.ComponentChildAttributes{
//			UID:       uid,
//			ParentUID: &rootUID,
//			Group:     &group,
//			Version:   "v1",
//			Kind:      "Test",
//			Namespace: &namespace,
//			Name:      "level1-component-" + string(rune('a'+i)),
//			State:     &state,
//		}
//
//		err := cache.SetComponent(component)
//		require.NoError(b, err)
//
//		// Create 5 second-level children for each first-level child
//		for j := 0; j < 5; j++ {
//			childUID := "level2-uid-" + string(rune('a'+i)) + string(rune('0'+j))
//			childComponent := client.ComponentChildAttributes{
//				UID:       childUID,
//				ParentUID: &uid,
//				Group:     &group,
//				Version:   "v1",
//				Kind:      "Test",
//				Namespace: &namespace,
//				Name:      "level2-component-" + string(rune('a'+i)) + string(rune('0'+j)),
//				State:     &state,
//			}
//
//			err := cache.SetComponent(childComponent)
//			require.NoError(b, err)
//
//			// Create 3 third-level children for each second-level child
//			for k := 0; k < 3; k++ {
//				grandchildUID := "level3-uid-" + string(rune('a'+i)) + string(rune('0'+j)) + string(rune('0'+k))
//				grandchildComponent := client.ComponentChildAttributes{
//					UID:       grandchildUID,
//					ParentUID: &childUID,
//					Group:     &group,
//					Version:   "v1",
//					Kind:      "Test",
//					Namespace: &namespace,
//					Name:      "level3-component-" + string(rune('a'+i)) + string(rune('0'+j)) + string(rune('0'+k)),
//					State:     &state,
//				}
//
//				err := cache.SetComponent(grandchildComponent)
//				require.NoError(b, err)
//			}
//		}
//	}
//}
//
//// BenchmarkMemoryCache runs all benchmarks for the in-memory cache
//func BenchmarkMemoryCache(b *testing.B) {
//	// Make sure we start with a clean state
//	if cache := db.GetComponentCache(); cache != nil {
//		cache.Close()
//	}
//
//	// Initialize the cache once for all benchmarks
//	db.Init()
//
//	// Store the cache reference to avoid race conditions
//	cache := db.GetComponentCache()
//	defer cache.Close()
//
//	// Run the SetComponent benchmark
//	b.Run("SetComponent", func(b *testing.B) {
//		state := client.ComponentState("Healthy")
//		group := testGroup
//		namespace := testNamespace
//
//		b.ResetTimer()
//		i := 0
//		for b.Loop() {
//			uid := "bench-uid-" + string(rune('a'+i%26))
//			component := client.ComponentChildAttributes{
//				UID:       uid,
//				Group:     &group,
//				Version:   "v1",
//				Kind:      "Test",
//				Namespace: &namespace,
//				Name:      "bench-component-" + string(rune('a'+i%26)),
//				State:     &state,
//			}
//			i++
//
//			err := cache.SetComponent(component)
//			require.NoError(b, err)
//		}
//	})
//
//	// Setup test data for the remaining benchmarks
//	setupTestData(b, cache)
//
//	// Run the ComponentChildren benchmark
//	b.Run("ComponentChildren", func(b *testing.B) {
//		b.ResetTimer()
//		for b.Loop() {
//			children, err := cache.ComponentChildren("root-uid")
//			require.NoError(b, err)
//			require.NotEmpty(b, children)
//		}
//	})
//
//	// Run the DeleteComponent benchmark
//	b.Run("DeleteComponent", func(b *testing.B) {
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			// DeleteComponent a level 1 component (which should cascade to its children)
//			err := cache.DeleteComponent("level1-uid-" + string(rune('a'+i%10)))
//			require.NoError(b, err)
//		}
//	})
//
//	// Setup test data again for the combined benchmark
//	setupTestData(b, cache)
//
//	// Run the SetComponentAndComponentChildren benchmark
//	b.Run("SetComponentAndComponentChildren", func(b *testing.B) {
//		state := client.ComponentState("Healthy")
//		group := testGroup
//		namespace := testNamespace
//
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			// Add a new component
//			uid := "bench-uid-" + string(rune('a'+i%26))
//			parentUID := "level1-uid-" + string(rune('a'+i%10))
//			component := client.ComponentChildAttributes{
//				UID:       uid,
//				ParentUID: &parentUID,
//				Group:     &group,
//				Version:   "v1",
//				Kind:      "Test",
//				Namespace: &namespace,
//				Name:      "bench-component-" + string(rune('a'+i%26)),
//				State:     &state,
//			}
//
//			err := cache.SetComponent(component)
//			require.NoError(b, err)
//
//			// Retrieve children
//			children, err := cache.ComponentChildren("root-uid")
//			require.NoError(b, err)
//			require.NotEmpty(b, children)
//		}
//	})
//}
//
//// BenchmarkFileCache runs all benchmarks for the file-based cache
//func BenchmarkFileCache(b *testing.B) {
//	// Make sure we start with a clean state
//	if cache := db.GetComponentCache(); cache != nil {
//		cache.Close()
//	}
//	_ = os.Remove(benchDBFile)
//
//	// Initialize the cache once for all benchmarks
//	db.Init(db.WithMode(db.CacheModeFile), db.WithFilePath(benchDBFile))
//
//	// Store the cache reference to avoid race conditions
//	cache := db.GetComponentCache()
//	defer func() {
//		if cache != nil {
//			cache.Close()
//		}
//		_ = os.Remove(benchDBFile)
//	}()
//
//	// Run the SetComponent benchmark
//	b.Run("SetComponent", func(b *testing.B) {
//		state := client.ComponentState("Healthy")
//		group := testGroup
//		namespace := testNamespace
//
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			uid := "bench-uid-" + string(rune('a'+i%26))
//			component := client.ComponentChildAttributes{
//				UID:       uid,
//				Group:     &group,
//				Version:   "v1",
//				Kind:      "Test",
//				Namespace: &namespace,
//				Name:      "bench-component-" + string(rune('a'+i%26)),
//				State:     &state,
//			}
//
//			err := cache.SetComponent(component)
//			require.NoError(b, err)
//		}
//	})
//
//	// Setup test data for the remaining benchmarks
//	setupTestData(b, cache)
//
//	// Run the ComponentChildren benchmark
//	b.Run("ComponentChildren", func(b *testing.B) {
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			children, err := cache.ComponentChildren("root-uid")
//			require.NoError(b, err)
//			require.NotEmpty(b, children)
//		}
//	})
//
//	// Run the DeleteComponent benchmark
//	b.Run("DeleteComponent", func(b *testing.B) {
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			// DeleteComponent a level 1 component (which should cascade to its children)
//			err := cache.DeleteComponent("level1-uid-" + string(rune('a'+i%10)))
//			require.NoError(b, err)
//		}
//	})
//
//	// Setup test data again for the combined benchmark
//	setupTestData(b, cache)
//
//	// Run the SetAndChildren benchmark
//	b.Run("SetAndChildren", func(b *testing.B) {
//		state := client.ComponentState("Healthy")
//		group := testGroup
//		namespace := testNamespace
//
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			// Add a new component
//			uid := "bench-uid-" + string(rune('a'+i%26))
//			parentUID := "level1-uid-" + string(rune('a'+i%10))
//			component := client.ComponentChildAttributes{
//				UID:       uid,
//				ParentUID: &parentUID,
//				Group:     &group,
//				Version:   "v1",
//				Kind:      "Test",
//				Namespace: &namespace,
//				Name:      "bench-component-" + string(rune('a'+i%26)),
//				State:     &state,
//			}
//
//			err := cache.SetComponent(component)
//			require.NoError(b, err)
//
//			// Retrieve children
//			children, err := cache.ComponentChildren("root-uid")
//			require.NoError(b, err)
//			require.NotEmpty(b, children)
//		}
//	})
//}
