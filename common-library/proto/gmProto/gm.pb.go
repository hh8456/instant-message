// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: gm.proto

package gmProto

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
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
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type GMSetPlayerCoin struct {
	RoleID               int64    `protobuf:"varint,1,opt,name=roleID,proto3" json:"roleID,omitempty"`
	Num                  int32    `protobuf:"varint,2,opt,name=num,proto3" json:"num,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GMSetPlayerCoin) Reset()         { *m = GMSetPlayerCoin{} }
func (m *GMSetPlayerCoin) String() string { return proto.CompactTextString(m) }
func (*GMSetPlayerCoin) ProtoMessage()    {}
func (*GMSetPlayerCoin) Descriptor() ([]byte, []int) {
	return fileDescriptor_9e843c9cc9a3a239, []int{0}
}
func (m *GMSetPlayerCoin) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GMSetPlayerCoin) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GMSetPlayerCoin.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GMSetPlayerCoin) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GMSetPlayerCoin.Merge(m, src)
}
func (m *GMSetPlayerCoin) XXX_Size() int {
	return m.Size()
}
func (m *GMSetPlayerCoin) XXX_DiscardUnknown() {
	xxx_messageInfo_GMSetPlayerCoin.DiscardUnknown(m)
}

var xxx_messageInfo_GMSetPlayerCoin proto.InternalMessageInfo

func (m *GMSetPlayerCoin) GetRoleID() int64 {
	if m != nil {
		return m.RoleID
	}
	return 0
}

func (m *GMSetPlayerCoin) GetNum() int32 {
	if m != nil {
		return m.Num
	}
	return 0
}

type GMSetPlayerDiamond struct {
	RoleID               int64    `protobuf:"varint,1,opt,name=roleID,proto3" json:"roleID,omitempty"`
	Num                  int32    `protobuf:"varint,2,opt,name=num,proto3" json:"num,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GMSetPlayerDiamond) Reset()         { *m = GMSetPlayerDiamond{} }
func (m *GMSetPlayerDiamond) String() string { return proto.CompactTextString(m) }
func (*GMSetPlayerDiamond) ProtoMessage()    {}
func (*GMSetPlayerDiamond) Descriptor() ([]byte, []int) {
	return fileDescriptor_9e843c9cc9a3a239, []int{1}
}
func (m *GMSetPlayerDiamond) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GMSetPlayerDiamond) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GMSetPlayerDiamond.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GMSetPlayerDiamond) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GMSetPlayerDiamond.Merge(m, src)
}
func (m *GMSetPlayerDiamond) XXX_Size() int {
	return m.Size()
}
func (m *GMSetPlayerDiamond) XXX_DiscardUnknown() {
	xxx_messageInfo_GMSetPlayerDiamond.DiscardUnknown(m)
}

var xxx_messageInfo_GMSetPlayerDiamond proto.InternalMessageInfo

func (m *GMSetPlayerDiamond) GetRoleID() int64 {
	if m != nil {
		return m.RoleID
	}
	return 0
}

func (m *GMSetPlayerDiamond) GetNum() int32 {
	if m != nil {
		return m.Num
	}
	return 0
}

func init() {
	proto.RegisterType((*GMSetPlayerCoin)(nil), "gmProto.GMSetPlayerCoin")
	proto.RegisterType((*GMSetPlayerDiamond)(nil), "gmProto.GMSetPlayerDiamond")
}

func init() { proto.RegisterFile("gm.proto", fileDescriptor_9e843c9cc9a3a239) }

var fileDescriptor_9e843c9cc9a3a239 = []byte{
	// 134 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x48, 0xcf, 0xd5, 0x2b,
	0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x4f, 0xcf, 0x0d, 0x00, 0x31, 0x94, 0xac, 0xb9, 0xf8, 0xdd,
	0x7d, 0x83, 0x53, 0x4b, 0x02, 0x72, 0x12, 0x2b, 0x53, 0x8b, 0x9c, 0xf3, 0x33, 0xf3, 0x84, 0xc4,
	0xb8, 0xd8, 0x8a, 0xf2, 0x73, 0x52, 0x3d, 0x5d, 0x24, 0x18, 0x15, 0x18, 0x35, 0x98, 0x83, 0xa0,
	0x3c, 0x21, 0x01, 0x2e, 0xe6, 0xbc, 0xd2, 0x5c, 0x09, 0x26, 0x05, 0x46, 0x0d, 0xd6, 0x20, 0x10,
	0x53, 0xc9, 0x8e, 0x4b, 0x08, 0x49, 0xb3, 0x4b, 0x66, 0x62, 0x6e, 0x7e, 0x5e, 0x0a, 0xf1, 0xfa,
	0x9d, 0x04, 0x4e, 0x3c, 0x92, 0x63, 0xbc, 0xf0, 0x48, 0x8e, 0xf1, 0xc1, 0x23, 0x39, 0xc6, 0x19,
	0x8f, 0xe5, 0x18, 0x92, 0xd8, 0xc0, 0xce, 0x33, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0xd9, 0x73,
	0xfd, 0x18, 0xaa, 0x00, 0x00, 0x00,
}

func (m *GMSetPlayerCoin) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GMSetPlayerCoin) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GMSetPlayerCoin) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Num != 0 {
		i = encodeVarintGm(dAtA, i, uint64(m.Num))
		i--
		dAtA[i] = 0x10
	}
	if m.RoleID != 0 {
		i = encodeVarintGm(dAtA, i, uint64(m.RoleID))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *GMSetPlayerDiamond) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GMSetPlayerDiamond) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GMSetPlayerDiamond) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Num != 0 {
		i = encodeVarintGm(dAtA, i, uint64(m.Num))
		i--
		dAtA[i] = 0x10
	}
	if m.RoleID != 0 {
		i = encodeVarintGm(dAtA, i, uint64(m.RoleID))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintGm(dAtA []byte, offset int, v uint64) int {
	offset -= sovGm(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *GMSetPlayerCoin) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.RoleID != 0 {
		n += 1 + sovGm(uint64(m.RoleID))
	}
	if m.Num != 0 {
		n += 1 + sovGm(uint64(m.Num))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *GMSetPlayerDiamond) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.RoleID != 0 {
		n += 1 + sovGm(uint64(m.RoleID))
	}
	if m.Num != 0 {
		n += 1 + sovGm(uint64(m.Num))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovGm(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozGm(x uint64) (n int) {
	return sovGm(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *GMSetPlayerCoin) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGm
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
			return fmt.Errorf("proto: GMSetPlayerCoin: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GMSetPlayerCoin: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field RoleID", wireType)
			}
			m.RoleID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGm
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.RoleID |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Num", wireType)
			}
			m.Num = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGm
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Num |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipGm(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthGm
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthGm
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *GMSetPlayerDiamond) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGm
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
			return fmt.Errorf("proto: GMSetPlayerDiamond: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GMSetPlayerDiamond: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field RoleID", wireType)
			}
			m.RoleID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGm
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.RoleID |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Num", wireType)
			}
			m.Num = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGm
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Num |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipGm(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthGm
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthGm
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipGm(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowGm
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
					return 0, ErrIntOverflowGm
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGm
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
				return 0, ErrInvalidLengthGm
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthGm
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowGm
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipGm(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthGm
				}
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthGm = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowGm   = fmt.Errorf("proto: integer overflow")
)
