- Struct's size depends on
	- Field's size
	- Order of the fields (alignment + Padding)
	- OS (Word Length -- 8 bytes on 64 bit system & 4 bytes for 32 bit system)

Struct's with the same content, but efficient order might
- occupy different size in mem
- speed up execution

`mspans`:
- data on the heap is stored in chunks, called `mspans`.
- `mspans` have differenty size classes. There are 68 of them: (in bytes) 0, 8, 16, 32, 48, 64, 80, 96 ... upto 32kb.

more info: https://www.youtube.com/watch?v=HPc-C0bx3kg&list=PLtoVuM73AmsIf99_fXLq_ehe2tpGVJQiF&index=4