package render

import (
	"fmt"

	"github.com/gogpu/wgpu"
)

type worldMeshVersion struct {
	mesh    *WorldMesh
	version uint64
}

// gpuWorldMeshBatch combines the currently visible meshes sharing a render key.
// Keep their order, including duplicate submissions, so merging draws preserves
// depth and blending results. Visibility changes rebuild only affected batches.
type gpuWorldMeshBatch struct {
	vertexBuf   dynamicGPUBuffer
	indexBuf    dynamicGPUBuffer
	indexCount  uint32
	meshes      []worldMeshVersion
	width       int
	height      int
	lightWidth  int
	lightHeight int
}

func (b *gpuWorldMeshBatch) matches(meshes []*WorldMesh, width, height, lightWidth, lightHeight int) bool {
	if b.width != width || b.height != height || b.lightWidth != lightWidth || b.lightHeight != lightHeight || len(b.meshes) != len(meshes) {
		return false
	}
	for i, mesh := range meshes {
		if b.meshes[i].mesh != mesh || b.meshes[i].version != mesh.version {
			return false
		}
	}
	return true
}

func (b *gpuWorldMeshBatch) remember(meshes []*WorldMesh, width, height, lightWidth, lightHeight int) {
	clear(b.meshes)
	b.meshes = reserveSlice(b.meshes, len(meshes))
	for _, mesh := range meshes {
		b.meshes = append(b.meshes, worldMeshVersion{mesh: mesh, version: mesh.version})
	}
	b.width, b.height = width, height
	b.lightWidth, b.lightHeight = lightWidth, lightHeight
}

func (b *gpuWorldMeshBatch) release() {
	if b.vertexBuf.buf != nil {
		b.vertexBuf.buf.Release()
	}
	if b.indexBuf.buf != nil {
		b.indexBuf.buf.Release()
	}
	*b = gpuWorldMeshBatch{}
}

func (r *gpuRenderer) pruneWorldMeshBatchCache() {
	// Retain only visible render keys, including across map changes. Otherwise
	// old batches would keep both their GPU buffers and entire map textures alive.
	for key, batch := range r.worldMeshBatchCache {
		if _, visible := r.worldMeshBatchByKey[key]; !visible {
			batch.release()
			delete(r.worldMeshBatchCache, key)
		}
	}
}

func (r *gpuRenderer) ensureWorldMeshBatch(batch worldMeshBatch) (*gpuWorldMeshBatch, error) {
	width, height := batch.key.texture.Bounds().Dx(), batch.key.texture.Bounds().Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("world mesh batch texture has invalid size")
	}
	lw, lh := lightTextureSize(batch.key.lightTexture, width, height)
	if r.worldMeshBatchCache == nil {
		r.worldMeshBatchCache = make(map[drawBatchKey]*gpuWorldMeshBatch)
	}
	cached := r.worldMeshBatchCache[batch.key]
	if cached == nil {
		cached = &gpuWorldMeshBatch{}
		r.worldMeshBatchCache[batch.key] = cached
	}
	if cached.matches(batch.meshes, width, height, lw, lh) {
		return cached, nil
	}
	floats, indices := worldMeshBatchGPUData(r.worldMeshBatchFloats, r.worldMeshBatchIndices, batch, width, height, lw, lh)
	r.worldMeshBatchFloats, r.worldMeshBatchIndices = floats, indices
	if _, err := r.dynamicBuffer(&cached.vertexBuf, "goro-world-mesh-batch-vertices", len(floats)*4,
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst, floatBytes(floats)); err != nil {
		return nil, err
	}
	if _, err := r.dynamicBuffer(&cached.indexBuf, "goro-world-mesh-batch-indices", len(indices)*4,
		wgpu.BufferUsageIndex|wgpu.BufferUsageCopyDst, u32Bytes(indices)); err != nil {
		return nil, err
	}
	cached.indexCount = uint32(len(indices))
	cached.remember(batch.meshes, width, height, lw, lh)
	return cached, nil
}

func worldMeshBatchGPUData(floats []float32, indices []uint32, batch worldMeshBatch, width, height, lightWidth, lightHeight int) ([]float32, []uint32) {
	vertexCount, indexCount := 0, 0
	for _, mesh := range batch.meshes {
		vertexCount += len(mesh.vertices)
		indexCount += len(mesh.indices)
	}
	floats = reserveSlice(floats, vertexCount*worldVertexFloatCount)
	indices = reserveSlice(indices, indexCount)
	for _, mesh := range batch.meshes {
		floats, indices = appendWorldCommand(floats, indices, WorldCommand{
			Vertices: mesh.vertices,
			Indices:  mesh.indices,
			Options:  mesh.options,
		}, width, height, lightWidth, lightHeight)
	}
	return floats, indices
}
