;redcode-94
;name Carpet
;author corewar demo
; Advances a pointer and plants a DAT bomb on every fifth address.
step    EQU     5
        ORG     loop
loop    ADD.AB  #step, ptr
        MOV.I   bomb, @ptr
        JMP     loop
ptr     DAT     #0, #1
bomb    DAT     #0, #0
