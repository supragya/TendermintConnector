// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: tendermint/types/block.proto

package types

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type Block struct {
	Header     Header       `protobuf:"bytes,1,opt,name=header,proto3" json:"header"`
	Data       Data         `protobuf:"bytes,2,opt,name=data,proto3" json:"data"`
	Evidence   EvidenceList `protobuf:"bytes,3,opt,name=evidence,proto3" json:"evidence"`
	LastCommit *Commit      `protobuf:"bytes,4,opt,name=last_commit,json=lastCommit,proto3" json:"last_commit,omitempty"`
}

func (m *Block) Reset()         { *m = Block{} }
func (m *Block) String() string { return proto.CompactTextString(m) }
func (*Block) ProtoMessage()    {}
func (*Block) Descriptor() ([]byte, []int) {
	return fileDescriptor_70840e82f4357ab1, []int{0}
}
func (m *Block) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Block) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Block.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Block) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Block.Merge(m, src)
}
func (m *Block) XXX_Size() int {
	return m.Size()
}
func (m *Block) XXX_DiscardUnknown() {
	xxx_messageInfo_Block.DiscardUnknown(m)
}

var xxx_messageInfo_Block proto.InternalMessageInfo

func (m *Block) GetHeader() Header {
	if m != nil {
		return m.Header
	}
	return Header{}
}

func (m *Block) GetData() Data {
	if m != nil {
		return m.Data
	}
	return Data{}
}

func (m *Block) GetEvidence() EvidenceList {
	if m != nil {
		return m.Evidence
	}
	return EvidenceList{}
}

func (m *Block) GetLastCommit() *Commit {
	if m != nil {
		return m.LastCommit
	}
	return nil
}

func init() {

}

func init() {}

var fileDescriptor_70840e82f4357ab1 = []byte{
	// 294 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x91, 0x41, 0x4b, 0xc3, 0x30,
	0x18, 0x86, 0x1b, 0xad, 0x43, 0xb2, 0x8b, 0x14, 0x91, 0x32, 0x24, 0x13, 0x4f, 0x9e, 0x1a, 0x71,
	0x22, 0x78, 0x93, 0x4e, 0x41, 0xc4, 0xd3, 0xf0, 0xe4, 0x45, 0xd2, 0x34, 0xb4, 0xc1, 0x35, 0x29,
	0xcd, 0x37, 0x61, 0xff, 0xc2, 0x9f, 0xb5, 0xe3, 0x8e, 0x9e, 0x44, 0xda, 0xbb, 0xbf, 0x41, 0x9a,
	0xc6, 0x09, 0x2b, 0x5e, 0xca, 0x47, 0x9f, 0xf7, 0xf9, 0xf2, 0x92, 0xe0, 0x63, 0x10, 0x2a, 0x15,
	0x55, 0x21, 0x15, 0x50, 0x58, 0x96, 0xc2, 0xd0, 0x64, 0xae, 0xf9, 0x6b, 0x54, 0x56, 0x1a, 0x74,
	0x70, 0xf0, 0x47, 0x23, 0x4b, 0x47, 0x87, 0x99, 0xce, 0xb4, 0x85, 0xb4, 0x9d, 0xba, 0xdc, 0xa8,
	0xbf, 0xc5, 0x7e, 0x1d, 0x1d, 0xf7, 0xa8, 0x78, 0x93, 0xa9, 0x50, 0x5c, 0x74, 0x81, 0xd3, 0x6f,
	0x84, 0xf7, 0xe2, 0xf6, 0xd8, 0xe0, 0x0a, 0x0f, 0x72, 0xc1, 0x52, 0x51, 0x85, 0xe8, 0x04, 0x9d,
	0x0d, 0x2f, 0xc2, 0x68, 0xbb, 0x41, 0x74, 0x6f, 0x79, 0xec, 0xaf, 0x3e, 0xc7, 0xde, 0xcc, 0xa5,
	0x83, 0x73, 0xec, 0xa7, 0x0c, 0x58, 0xb8, 0x63, 0xad, 0xa3, 0xbe, 0x75, 0xcb, 0x80, 0x39, 0xc7,
	0x26, 0x83, 0x1b, 0xbc, 0xff, 0xdb, 0x22, 0xdc, 0xb5, 0x16, 0xe9, 0x5b, 0x77, 0x2e, 0xf1, 0x28,
	0x0d, 0x38, 0x7b, 0x63, 0x05, 0xd7, 0x78, 0x38, 0x67, 0x06, 0x5e, 0xb8, 0x2e, 0x0a, 0x09, 0xa1,
	0xff, 0x5f, 0xe1, 0xa9, 0xe5, 0x33, 0xdc, 0x86, 0xbb, 0x39, 0x4e, 0x57, 0x35, 0x41, 0xeb, 0x9a,
	0xa0, 0xaf, 0x9a, 0xa0, 0xf7, 0x86, 0x78, 0xeb, 0x86, 0x78, 0x1f, 0x0d, 0xf1, 0x9e, 0x1f, 0x32,
	0x09, 0xf9, 0x22, 0x89, 0xb8, 0x2e, 0xa8, 0x59, 0x94, 0x15, 0xcb, 0x96, 0x8c, 0x3e, 0x6d, 0x56,
	0x4e, 0xb5, 0x52, 0x82, 0x83, 0xae, 0x28, 0xcf, 0x99, 0x54, 0x86, 0x42, 0x31, 0xb9, 0xa4, 0xdd,
	0x7b, 0x6c, 0xdf, 0x72, 0x32, 0xb0, 0xff, 0x27, 0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0x90, 0x0b,
	0x6a, 0x66, 0xe4, 0x01, 0x00, 0x00,
}

func (m *Block) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Block) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Block) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.LastCommit != nil {
		{
			size, err := m.LastCommit.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintBlock(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	{
		size, err := m.Evidence.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintBlock(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x1a
	{
		size, err := m.Data.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintBlock(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x12
	{
		size, err := m.Header.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintBlock(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func encodeVarintBlock(dAtA []byte, offset int, v uint64) int {
	offset -= sovBlock(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Block) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Header.Size()
	n += 1 + l + sovBlock(uint64(l))
	l = m.Data.Size()
	n += 1 + l + sovBlock(uint64(l))
	l = m.Evidence.Size()
	n += 1 + l + sovBlock(uint64(l))
	if m.LastCommit != nil {
		l = m.LastCommit.Size()
		n += 1 + l + sovBlock(uint64(l))
	}
	return n
}

func sovBlock(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozBlock(x uint64) (n int) {
	return sovBlock(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Block) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowBlock
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Block: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Block: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Header", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlock
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBlock
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Header.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlock
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBlock
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Data.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Evidence", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlock
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBlock
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Evidence.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field LastCommit", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlock
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBlock
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.LastCommit == nil {
				m.LastCommit = &Commit{}
			}
			if err := m.LastCommit.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipBlock(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthBlock
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipBlock(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowBlock
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowBlock
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowBlock
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthBlock
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupBlock
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthBlock
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthBlock        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowBlock          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupBlock = fmt.Errorf("proto: unexpected end of group")
)