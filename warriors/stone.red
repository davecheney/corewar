;redcode-94
;name Stone
;author corewar demo
; A dwarf with a wide step, scattering bombs across the whole core.
step    EQU     3044
        ORG     loop
loop    ADD.AB  #step, bomb
        MOV.I   bomb, @bomb
        JMP     loop
bomb    DAT     #0, #0
