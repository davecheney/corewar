;redcode-94
;name Scanner
;author corewar demo
; Looks for non-empty cells and bombs them, avoiding its own code.
step    EQU     7
ptr     DAT     #0, #step
scan    ADD.AB  #step, ptr
        JMZ.F   scan, @ptr
        SLT.AB  #bomb-ptr, ptr
        JMP     scan
        MOV.I   bomb, @ptr
        JMP     scan
bomb    DAT     #0, #0
        END     scan
