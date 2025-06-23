import logging
from typing import List, Tuple

import numpy as np
from PIL import Image
from sklearn.neighbors import NearestNeighbors

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)


class ImageDitherer:

    def __init__(self):
        # Predefined color palettes
        self.palettes = {
            "grayscale_4": [
                (0, 0, 0),  # Black
                (85, 85, 85),  # Dark gray
                (170, 170, 170),  # Light gray
                (255, 255, 255),  # White
            ],
            "7-color": [  # converted from WaveShare 7.3inch e-Paper (F) lab values
                (49, 40, 56),  # lab(17.6, 8.3, -8.9) (DS, Dark state)
                (174, 173, 168),  # lab(70.6, -0.4, 2.4) (WS, White state)
                (57, 63, 104),  # lab(28, 9.2, -25) (BS, Blue state)
                (48, 101, 68),  # lab(38.3, -26, 13.4) (GS, Green state)
                (146, 61, 62),  # lab(37.6, 35.9, 17.4) (RS, Red state)
                (173, 160, 73),  # lab(65.5, -6.7, 46.4) (YS, Yellow state)
                (160, 83, 65),  # lab(44.4, 30, 24.9) (OS, Orange state)
            ],
            "7-color-2": [  # pure colors
                (0, 0, 0),  # Black
                (255, 255, 255),  # White
                (0, 0, 255),  # Blue
                (0, 255, 0),  # Green
                (255, 0, 0),  # Red
                (255, 255, 0),  # Yellow
                (255, 165, 0),  # Orange
            ],
        }

    def build_knn(self, palette: List[Tuple[int, int, int]]):
        """Build a NearestNeighbors model for a given palette."""
        self.knn_model = NearestNeighbors(n_neighbors=1, algorithm="auto")
        self.knn_colors = np.array(palette, dtype=np.float32)
        self.knn_model.fit(self.knn_colors)

    def knn_closest_color(self, pixel: Tuple[int, int, int]) -> Tuple[int, int, int]:
        """Use KNN model to find the closest palette color to a given pixel."""
        if not hasattr(self, "knn_model"):
            raise RuntimeError("KNN model not built. Call build_knn() first.")
        pixel_np = np.array([pixel], dtype=np.float32)
        _, indices = self.knn_model.kneighbors(pixel_np)
        return tuple(self.knn_colors[indices[0][0]].astype(int))

    def find_closest_color(
        self, pixel: Tuple[int, int, int], palette: List[Tuple[int, int, int]]
    ) -> Tuple[int, int, int]:
        """Find the closest color in the palette to the given pixel."""
        if not hasattr(self, "knn_model") or not np.array_equal(
            self.knn_colors, palette
        ):
            # Build the KNN model if it doesn't exist or palette has changed
            logging.info("Building KNN model for the palette.")
            self.build_knn(palette)
        closest_color = self.knn_closest_color(pixel)

        return closest_color

    def floyd_steinberg_dither(
        self, image: Image.Image, palette: List[Tuple[int, int, int]]
    ) -> Image.Image:
        """Apply Floyd-Steinberg dithering to the image with the given palette."""
        # Convert to RGB if not already
        if image.mode != "RGB":
            image = image.convert("RGB")

        # Convert to numpy array for easier manipulation
        img_array = np.array(image, dtype=np.float32)
        height, width = img_array.shape[:2]

        for y in range(height):
            for x in range(width):
                old_pixel = tuple(img_array[y, x].astype(int))
                new_pixel = self.find_closest_color(old_pixel, palette)

                # Set the new pixel
                img_array[y, x] = new_pixel

                # Calculate quantization error
                error = [old_pixel[i] - new_pixel[i] for i in range(3)]

                # Distribute error to neighboring pixels
                if x + 1 < width:
                    img_array[y, x + 1] += [e * 7 / 16 for e in error]

                if y + 1 < height:
                    if x > 0:
                        img_array[y + 1, x - 1] += [e * 3 / 16 for e in error]

                    img_array[y + 1, x] += [e * 5 / 16 for e in error]

                    if x + 1 < width:
                        img_array[y + 1, x + 1] += [e * 1 / 16 for e in error]

        # Clamp values to valid range
        img_array = np.clip(img_array, 0, 255)

        # Convert back to PIL Image
        return Image.fromarray(img_array.astype(np.uint8))

    def stucki_dither(
        self,
        image: Image.Image,
        palette: List[Tuple[int, int, int]],
        use_knn: bool = False,
    ) -> Image.Image:
        """Apply Stucki dithering to the image with the given palette."""
        if image.mode != "RGB":
            image = image.convert("RGB")

        img_array = np.array(image, dtype=np.float32)
        height, width = img_array.shape[:2]

        # Stucki diffusion matrix
        diffusion_matrix = [
            (1, 0, 8),
            (2, 0, 4),
            (-2, 1, 2),
            (-1, 1, 4),
            (0, 1, 8),
            (1, 1, 4),
            (2, 1, 2),
            (-2, 2, 1),
            (-1, 2, 2),
            (0, 2, 4),
            (1, 2, 2),
            (2, 2, 1),
        ]
        total_weight = 42.0

        for y in range(height):
            for x in range(width):
                old_pixel = tuple(img_array[y, x].astype(int))
                new_pixel = self.find_closest_color(old_pixel, palette)

                img_array[y, x] = new_pixel
                error = [old_pixel[i] - new_pixel[i] for i in range(3)]

                for dx, dy, weight in diffusion_matrix:
                    nx, ny = x + dx, y + dy
                    if 0 <= nx < width and 0 <= ny < height:
                        for c in range(3):
                            img_array[ny, nx, c] += error[c] * (weight / total_weight)

        img_array = np.clip(img_array, 0, 255)
        return Image.fromarray(img_array.astype(np.uint8))

    def resize_image(
        self,
        image: Image.Image,
        target_size: Tuple[int, int],
        maintain_aspect: bool = True,
        method: str = "fill",
    ) -> Image.Image:
        """Resize image to target size, optionally maintaining aspect ratio.

        Args:
            image: The input PIL Image
            target_size: The desired (width, height)
            maintain_aspect: Whether to maintain the aspect ratio
            method: Resizing method when maintaining aspect ratio:
                - "fit": Fit the image within the target size (default thumbnail behavior)
                - "fill": Fill the target size and crop excess (centered)
                - "cut": Cut/crop from center to exactly match target size

        Returns:
            Resized PIL Image
        """
        if not maintain_aspect:
            # Resize to exact dimensions
            return image.resize(target_size, Image.Resampling.LANCZOS)

        target_width, target_height = target_size

        if method == "fit":
            # Fit the image within the target size (maintains aspect ratio)
            resized = image.copy()
            resized.thumbnail(target_size, Image.Resampling.LANCZOS)
            return resized

        elif method == "fill":
            # Scale to fill and then crop excess (centered)
            img_width, img_height = image.size
            img_ratio = img_width / img_height
            target_ratio = target_width / target_height

            if target_ratio > img_ratio:  # Target is wider than image
                # Scale to match target width
                new_height = int(target_width / img_ratio)
                resized = image.resize(
                    (target_width, new_height), Image.Resampling.LANCZOS
                )
                # Center crop height
                top = (new_height - target_height) // 2
                bottom = top + target_height
                return resized.crop((0, top, target_width, bottom))
            else:  # Target is taller or same as image
                # Scale to match target height
                new_width = int(target_height * img_ratio)
                resized = image.resize(
                    (new_width, target_height), Image.Resampling.LANCZOS
                )
                # Center crop width
                left = (new_width - target_width) // 2
                right = left + target_width
                return resized.crop((left, 0, right, target_height))

        elif method == "cut":
            # Center cut/crop without resizing
            img_width, img_height = image.size

            # Check if we need to resize first (if image is smaller than target)
            if img_width < target_width or img_height < target_height:
                # Scale up the smaller dimension to match target
                scale = max(target_width / img_width, target_height / img_height)
                new_size = (int(img_width * scale), int(img_height * scale))
                image = image.resize(new_size, Image.Resampling.LANCZOS)
                img_width, img_height = image.size

            # Calculate crop coordinates
            left = (img_width - target_width) // 2
            top = (img_height - target_height) // 2
            right = left + target_width
            bottom = top + target_height

            return image.crop((left, top, right, bottom))

        else:
            raise ValueError(f"Unknown resize method: {method}")

    def process_image(
        self,
        input_image: Image.Image,
        target_size: Tuple[int, int] = (800, 480),
        palette: str | List[Tuple[int, int, int]] = "7-color",
        maintain_aspect: bool = True,
        dither_method: str = "stucki",
        resize_method: str = "fill",
    ) -> Image.Image:
        """Process an image: resize and apply dithering.

        Args:
            input_image: PIL Image to process
            output_image: Optional PIL Image to save results to
            target_size: Tuple of (width, height) to resize to
            palette_name: Name of predefined palette to use
            custom_palette: Custom color palette to use instead of named palette
            maintain_aspect: Whether to maintain aspect ratio when resizing

        Returns:
            The processed PIL Image object
        """
        # Get input image
        image = input_image.copy()
        logging.info(f"Processing image: {image.size[0]}x{image.size[1]}")

        # Resize image if target size is provided
        if target_size:
            resized_image = self.resize_image(
                image, target_size, maintain_aspect, method=resize_method
            )
            logging.info(f"Resized to: {resized_image.size[0]}x{resized_image.size[1]}")
        else:
            resized_image = image

        # Get palette
        if isinstance(palette, list):
            # Custom palette provided as a list of RGB tuples
            pass
        elif palette in self.palettes:
            palette = self.palettes[palette]
        else:
            raise ValueError(f"Unknown palette: {palette}")
        assert isinstance(
            palette, list
        ), "Palette must be a list of RGB tuples or a valid name"
        logging.info(f"Using palette: {palette} ({len(palette)} colors)")

        # Apply dithering
        if dither_method == "stucki":
            dithered_image = self.stucki_dither(resized_image, palette)
        elif dither_method == "floyd_steinberg":
            dithered_image = self.floyd_steinberg_dither(resized_image, palette)
        else:
            raise ValueError(f"Unknown dither method: {dither_method}")

        return dithered_image

    def list_palettes(self) -> None:
        """List available color palettes."""
        logging.info("Available palettes:")
        for name, colors in self.palettes.items():
            logging.info(f"  {name}: {len(colors)} colors")

    def create_palette_preview(
        self,
        palette_name: str,
    ) -> None:
        """Create a preview image showing the colors in a palette."""
        if palette_name not in self.palettes:
            logging.error(f"Unknown palette: {palette_name}")
            return

        palette = self.palettes[palette_name]

        # Create a grid showing the palette colors
        colors_per_row = min(8, len(palette))
        rows = (len(palette) + colors_per_row - 1) // colors_per_row

        cell_size = 50
        width = colors_per_row * cell_size
        height = rows * cell_size

        preview = Image.new("RGB", (width, height), (255, 255, 255))

        for i, color in enumerate(palette):
            row = i // colors_per_row
            col = i % colors_per_row

            x1 = col * cell_size
            y1 = row * cell_size
            x2 = x1 + cell_size
            y2 = y1 + cell_size

            # Fill the cell with the color
            for y in range(y1, y2):
                for x in range(x1, x2):
                    preview.putpixel((x, y), color)
        else:
            preview.show()


def parse_custom_palette(palette_str: str) -> List[Tuple[int, int, int]]:
    """Parse a custom palette from a string format like 'r,g,b;r,g,b;...'"""
    colors = []
    for color_str in palette_str.split(";"):
        try:
            r, g, b = map(int, color_str.split(","))
            colors.append((r, g, b))
        except ValueError:
            raise ValueError(f"Invalid color format: {color_str}")
    return colors


def main():
    from pathlib import Path

    image_path = Path("asset/example.jpg")  # Replace with your image path
    if not image_path.exists():
        logging.error(f"Image file not found: {image_path}")
        return
    # Parse command line arguments
    image = Image.open(image_path)

    ditherer = ImageDitherer()
    ditherer.list_palettes()
    dithered_image = ditherer.process_image(
        image,
        target_size=(800, 480),
        palette="7-color",
        maintain_aspect=True,
        dither_method="floyd_steinberg",  # or "stucki"
        resize_method="fill",  # or "fit", "cut"
    )
    dithered_image.show()  # Display the dithered image


if __name__ == "__main__":
    main()
