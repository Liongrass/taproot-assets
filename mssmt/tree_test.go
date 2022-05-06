package mssmt

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randKey() [hashSize]byte {
	var key [hashSize]byte
	_, _ = rand.Read(key[:])
	return key
}

func randLeaf() *LeafNode {
	valueLen := rand.Intn(math.MaxUint8) + 1
	value := make([]byte, valueLen)
	_, _ = rand.Read(value[:])
	sum := rand.Uint64()
	return NewLeafNode(value, sum)
}

func randTree(numLeaves int) (*Tree, map[[hashSize]byte]*LeafNode) {
	tree := NewTree(NewDefaultStore())
	leaves := make(map[[hashSize]byte]*LeafNode, numLeaves)
	for i := 0; i < numLeaves; i++ {
		key := randKey()
		leaf := randLeaf()
		tree.Insert(key, leaf)
		leaves[key] = leaf
	}
	return tree, leaves
}

// TestInsertion asserts that we can insert N leaves and retrieve them by their
// insertion key. Keys that do not exist within the tree should return an empty
// leaf.
func TestInsertion(t *testing.T) {
	t.Parallel()

	tree, leaves := randTree(10000)
	for key, leaf := range leaves {
		// The leaf was already inserted into the tree above, so verify
		// that we're able to look it up again.
		leafCopy := tree.Get(key)
		require.Equal(t, leaf, leafCopy)
	}

	emptyLeaf := tree.Get(randKey())
	require.True(t, emptyLeaf.IsEmpty())
}

// TestHistoryIndependence tests that given the same set of keys, two trees
// that insert the keys in an arbitrary order get the same root hash in the
// end.
func TestHistoryIndependence(t *testing.T) {
	t.Parallel()

	// Create a tree and insert 100 random leaves in to the tree.
	tree1, leaves := randTree(100)

	// Create a new empty tree, and iterate over the leaves (giving us a
	// randomized order) to insert them again in this new tree.
	tree2 := NewTree(NewDefaultStore())
	for key, leaf := range leaves {
		tree2.Insert(key, leaf)
	}

	// The root hash of both trees should be the same.
	require.Equal(t, *tree1.Root(), *tree2.Root())
}

// TestDeletion asserts that deleting all inserted leaves of a tree results in
// an empty tree.
func TestDeletion(t *testing.T) {
	t.Parallel()

	tree, leaves := randTree(10000)
	require.NotEqual(t, EmptyTree[0], tree.Root())
	for key := range leaves {
		_ = tree.Delete(key)
		emptyLeaf := tree.Get(key)
		require.True(t, emptyLeaf.IsEmpty())
	}
	require.Equal(t, EmptyTree[0], tree.Root())
}

// TestMerkleProof asserts that merkle proofs (inclusion and non-inclusion) for
// leaf nodes are constructed, compressed, decompressed, and verified properly.
func TestMerkleProof(t *testing.T) {
	t.Parallel()

	assertEqualAfterCompression := func(proof *Proof) {
		t.Helper()

		// Compressed proofs should never have empty nodes.
		compressedProof := proof.Compress()
		for _, node := range compressedProof.Nodes {
			for _, emptyNode := range EmptyTree {
				require.False(t, IsEqualNode(node, emptyNode))
			}
		}
		require.Equal(t, proof, compressedProof.Decompress())
	}

	// Create a random tree and verify each leaf's merkle proof.
	tree, leaves := randTree(1337)
	for key, leaf := range leaves {
		proof := tree.MerkleProof(key)
		assertEqualAfterCompression(proof)
		require.True(t, VerifyMerkleProof(key, leaf, proof, tree.Root()))
	}

	// Compute the proof for the first leaf and test some negative cases.
	for key, leaf := range leaves {
		proof := tree.MerkleProof(key)
		require.True(t, VerifyMerkleProof(key, leaf, proof, tree.Root()))

		// If we alter the proof's leaf sum, then the proof should no
		// longer be valid.
		leaf.sum++
		require.False(t, VerifyMerkleProof(key, leaf, proof, tree.Root()))
		leaf.sum--

		// If we delete the proof's leaf node from the tree, then it
		// should also no longer be valid.
		_ = tree.Delete(key)
		require.False(t, VerifyMerkleProof(key, leaf, proof, tree.Root()))
	}

	// Create a new leaf that will not be inserted in the tree. Computing
	// its proof should result in a non-inclusion proof (an empty leaf
	// exists at said key).
	nonExistentKey := randKey()
	nonExistentLeaf := randLeaf()
	proof := tree.MerkleProof(nonExistentKey)
	assertEqualAfterCompression(proof)
	require.False(t, VerifyMerkleProof(
		nonExistentKey, nonExistentLeaf, proof, tree.Root(),
	))
	require.True(t, VerifyMerkleProof(
		nonExistentKey, EmptyLeafNode, proof, tree.Root(),
	))
}
