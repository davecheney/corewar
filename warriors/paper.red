;
;    Warrior: Paper: A.K.A molerat
;  File name: paper.red
; Tournament: ICWST'90
;   Standard: CWS'88
;     Author: Scott Nelson
;
start   spl      1         1
        mov     -1         0
        mov     -1         0
;       sub    # 1         count       ;add these two lines to make it work
;       jmn      0         count       ;under '86 rules.
        spl      paper4
        spl      paper3
        spl      paper2
        jmp      paper1
count   dat                5
src1    mov    # 10        10
        sub    < 203       204
paper1  mov    < src1    < dest1
        mov    < src1    < dest1
        spl      back1     back1
        mov      bomb1   < 200
dest1   jmz    @ dest1     6101
back1   jmz      src1      src1
        mov      0        -1
bomb1   dat               -10
        dat                1
        dat                1
        dat                1
src2    mov    # 10        10
        add    < 203       204
paper2  mov    < src2    < dest2
        mov    < src2    < dest2
        spl      back2     back2
        mov      bomb2   < 200
dest2   jmz    @ dest2     6301
back2   jmz      src2      src2
        mov      0        -1
bomb2   dat               -10
        dat                1
        dat                1
        dat                1
        mov    # 10        10
        sub    < 203       204
paper3  mov    <-2       < 4
        mov    <-3       < 3
        spl      3         3
        mov      4       < 200
        jmz    @ 0         6501
        jmz     -7        -7
        mov      0        -1
        dat               -10
        dat                1
        dat                1
        dat                1
        mov    # 10        10
        add    < 203       204
paper4  mov    <-2       < 4
        mov    <-3       < 3
        spl      3         3
        mov      4       < 200
        jmz    @ 0         6701
        jmz     -7        -7
        mov      0        -1
        dat               -10
        end      start
