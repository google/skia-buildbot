package sensors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClose_WhenOpen_Success(t *testing.T) {

	p := fakeSerialPort{}

	assert.False(t, p.closed)
	require.NoError(t, p.Close())
	assert.True(t, p.closed)
}

func TestClose_WhenClosed_ReturnsError(t *testing.T) {

	p := fakeSerialPort{}

	assert.False(t, p.closed)

	require.NoError(t, p.Close())
	require.Error(t, p.Close())
}

func TestFlush_WhenOpen_Success(t *testing.T) {

	p := fakeSerialPort{}

	require.NoError(t, p.Flush())
	require.NoError(t, p.Close())
}

func TestFlush_WhenClosed_ReturnsError(t *testing.T) {

	p := fakeSerialPort{}

	require.NoError(t, p.Close())
	assert.Error(t, p.Flush())
}

func TestRead_SuccessiveReads_ContinuesWherePreviousEnded(t *testing.T) {

	p := fakeSerialPort{}
	p.setReadData(0, 1, 2, 3, 4, 5, 6, 7)

	b := make([]byte, 3)
	nr, err := p.Read(b)
	require.NoError(t, err)
	assert.Equal(t, 3, nr)
	assert.Equal(t, []byte{0, 1, 2}, b)

	b = make([]byte, 5)
	nr, err = p.Read(b)
	require.NoError(t, err)
	assert.Equal(t, 5, nr)
	assert.Equal(t, []byte{3, 4, 5, 6, 7}, b)
}

func TestRead_ReadNothing_Success(t *testing.T) {

	p := fakeSerialPort{}
	p.setReadData(0, 1, 2, 3, 4, 5, 6, 7)

	b := []byte{}
	nr, err := p.Read(b)
	require.NoError(t, err)
	assert.Equal(t, 0, nr)
	assert.Equal(t, []byte{}, b)
}

func TestRead_WhenClosed_ReturnsError(t *testing.T) {

	p := fakeSerialPort{}
	p.setReadData(0, 1, 2, 3, 4, 5, 6, 7)
	err := p.Close()
	require.NoError(t, err)

	b := make([]byte, 3)
	nr, err := p.Read(b)
	assert.Error(t, err)
	assert.Equal(t, 0, nr)
	assert.Equal(t, []byte{0x0, 0x0, 0x0}, b)
}

func TestRead_MoreThanAvail_ReturnOnlyAvailable(t *testing.T) {

	p := fakeSerialPort{}
	p.setReadData(0, 1, 2)

	b := make([]byte, 5)
	nr, err := p.Read(b)
	require.NoError(t, err)
	assert.Equal(t, 3, nr)
	assert.Equal(t, []byte{0, 1, 2, 0, 0}, b)
}

func TestWrite_MultipleWrites_BytesArrayAppended(t *testing.T) {

	p := fakeSerialPort{}
	nb, err := p.Write([]byte{0, 1, 2})
	require.NoError(t, err)
	assert.Equal(t, 3, nb)
	assert.Equal(t, []byte{0, 1, 2}, p.writtenData)

	nb, err = p.Write([]byte{7, 8})
	require.NoError(t, err)
	assert.Equal(t, 2, nb)
	assert.Equal(t, []byte{0, 1, 2, 7, 8}, p.writtenData)
}

func TestWrite_EmptyData_Success(t *testing.T) {

	p := fakeSerialPort{}
	nb, err := p.Write([]byte{0, 1, 2})
	require.NoError(t, err)
	assert.Equal(t, 3, nb)
	assert.Equal(t, []byte{0, 1, 2}, p.writtenData)

	nb, err = p.Write([]byte{})
	require.NoError(t, err)
	assert.Zero(t, nb)
	assert.Equal(t, []byte{0, 1, 2}, p.writtenData)
}

func TestWrite_WhenClosed_ReturnsError(t *testing.T) {

	p := fakeSerialPort{}

	nb, err := p.Write([]byte{1, 2})
	require.NoError(t, err)
	assert.Equal(t, 2, nb)
	assert.Equal(t, []byte{1, 2}, p.writtenData)

	require.NoError(t, p.Close())

	nb, err = p.Write([]byte{7, 8})
	assert.Error(t, err)
	assert.Zero(t, nb)
	assert.Equal(t, []byte{1, 2}, p.writtenData)
}
