tui-commander

Las barras de prograso
En el mensaje ETA: Calculating... debe mostrar algun progreso, tarda hasta que completa todo en un instante.
las filas con % pueden ser mas anchas, debe adaptarse al cuadro de dialogo que debe tener una ancho minimo establecido.
F2 no realiza ninguna accion.



la barra con % puede ser mas anchas
El calculo algo no funciona bien, muestra cantidad de bytes que va contando, parece que en el medio, algo copia, pero no lo muestra en ninguna de las barras de %



Hagamos un cambio.
Por ahora que la barra de progreso solo muestre nombre de archivos y cuantos archivos va copiando.
Una simplificion que luego abordaremos, no hace a lo funcional bloqueante si no está correcto.


Ahora si reporta los archivos en forma simplificada al copiar.
Pero esas 2 lineas podrian estar centradas en el cuadro de menu contextual.
la aplicacion Debe Iniciar maximizado.


1. si, abandone
2. al comenzar a probar, con presionar Ctrl+F la pantalla se puso toda en un color amarillo que no me permiter leer que opciones tengo.
3. Antes de continuar corrijamos esta etapa


Seleccion de a 1 archivo con space está funciona bien
La inversion de lo seleccionado con * funciona bien
Seleccionar unidad con Alt+F1 y F2 por panel funciona bien. Pero debería cambiar a panel activo el que se ha seleccionado unidad, al menos así lo hace total commander
El espacio reservado para los nombres puede ser mas ancho, como 20 caracteres mas. Y debería guardar esa proporcionalidad cuando la ventana no está maximizada. o ver de que forma tratar las proporciones.
si los nombres de los items (archivos o carpetas es muy largo, )
Como lo estoy lanzando desde el explorador de windows, la ventna debería iniciar maximizada por defecto.


Selector de unidad coloca el foco en el panel correcto.
Acabo de notar que la fecha de las carpetas siempre muestra la misma, no es la correcta.
Habiendo selecion multiple de archivos copia el archivo en el cual está la banda de color, no los archivos seleccionados. Si hay archivos seleccionados solo debe copiar los seleccionados. Lo mismo para mover y borrar.
En el cuadro de confirmacion, cuando es seleccion multiple no debe mostrar los nombres, como mucho la cantidad de items a realizar la accion.


Copia solo los items de seleccion multiplo.
Si hay texto seleccionado y se presiona escape, deben deseleccionarse todos los items.
copiar de a uno sigue funcionando, no necesita confirmacion si se copia de a uno, sin seleccion.
las carpetas ahora muestran su fecha
Que tan costosos es que elimine al portapapeles?
Analizar que sea configurable pero una sea por default.
En la carpeta d:\Desarrollo\tui-commander\docs\imgs\ tiene imagenes del norton commander para compender su branding.
Tambien tenía algunos menus desplegables.
Al salir con F10 que pida confirmacion, por default No. Que se pueda cambiar opcion con flechas o barra espaciadora


Analizar la posibilidad
Si los datos estan en una unidad fija, que vaya a la papelera, si la unidad es removible, se elimina definitivamente.

----



