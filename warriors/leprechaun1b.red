;redcode-icws
;name Leprechaun 1b
;author Anders Ivner
;strategy Combined Bomber/B-scanner, giving a total speed of 67% of c
;strategy coresize 8192 version
;assert CORESIZE==8192

offset  equ     5048

boot	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	<spt,		<dpt
	mov	bomb2+1,	bomb+17+2500
spt	jmp	start+2500,	bomb2+1
dpt     dat     #bomb2+2501

loop	add	#offset,	eye
start	mov	bomb,		@eye
eye	jmz	loop,		@offset+test+20
test    mov     eye,            @dp1
	mov	bomb2,		@eye-20
dp1     mov     bomb,           <eye-20
pt	jmn	loop,		test
bomb	spl	0,		<-20
        mov     16,             <test
bomb2	jmp	-1,		0

	dat	<-21,		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1
	dat		#0
	dat		#1
	dat		#1
	dat		#1
	dat		#0

	dat		#1
	dat		#1
	dat		#1

	end	boot
