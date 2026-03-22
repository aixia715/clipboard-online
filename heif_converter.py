#!/usr/bin/env python3
"""
HEIF/HEIC 图片转换工具
将 HEIF/HEIC 格式转换为 PNG 或 JPG
"""

import sys
import os
from pathlib import Path


def convert_heif_to_image(input_path: str, output_path: str) -> int:
    """
    将 HEIF/HEIC 图片转换为 PNG 或 JPG
    
    Args:
        input_path: 输入文件路径
        output_path: 输出文件路径
        
    Returns:
        成功返回 0，失败返回 1
    """
    try:
        import PIL
        from PIL import Image
        import pillow_heif
        
    except ImportError as e:
        print(f"错误: 缺少必要的库。请运行: pip install pillow pillow-heif", file=sys.stderr)
        print(f"详细信息: {e}", file=sys.stderr)
        return 1
    
    # 检查输入文件是否存在
    input_file = Path(input_path)
    if not input_file.exists():
        print(f"错误: 输入文件不存在: {input_path}", file=sys.stderr)
        return 1
    
    # 检查输入文件格式
    input_ext = input_file.suffix.lower()
    if input_ext not in ['.heif', '.heic']:
        print(f"错误: 不支持的输入格式。仅支持 .heif/.heic，得到: {input_ext}", file=sys.stderr)
        return 1
    
    # 检查输出文件格式
    output_file = Path(output_path)
    output_ext = output_file.suffix.lower()
    if output_ext not in ['.png', '.jpg', '.jpeg']:
        print(f"错误: 不支持的输出格式。仅支持 .png/.jpg/.jpeg，得到: {output_ext}", file=sys.stderr)
        return 1
    
    # 创建输出目录（如果不存在）
    try:
        output_file.parent.mkdir(parents=True, exist_ok=True)
    except Exception as e:
        print(f"错误: 无法创建输出目录: {output_file.parent}", file=sys.stderr)
        print(f"详细信息: {e}", file=sys.stderr)
        return 1
    
    try:
        # 注册 HEIF 打开器
        pillow_heif.register_heif_opener()
        
        # 打开 HEIF 图片
        img = Image.open(input_file)
        
        # 转换为 RGB 或 RGBA（取决于是否需要保留透明度）
        if output_ext.lower() == '.png':
            # PNG 支持透明度，保留 RGBA
            if img.mode in ('RGBA', 'LA', 'PA'):
                # 已经有透明通道，保持不变
                img_converted = img.convert('RGBA')
            else:
                # 转换为 RGBA（即使原图无透明通道）
                img_converted = img.convert('RGBA')
        else:
            # JPG 不支持透明度，转换为 RGB
            if img.mode in ('RGBA', 'LA', 'PA'):
                # 创建白色背景
                background = Image.new('RGB', img.size, (255, 255, 255))
                # 转换原图为 RGBA
                img_rgba = img.convert('RGBA')
                # 使用 alpha 通道作为蒙版合成
                background.paste(img_rgba, mask=img_rgba.split()[3])
                img_converted = background
            else:
                # 直接转换为 RGB
                img_converted = img.convert('RGB')
        
        # 保存文件
        if output_ext.lower() == '.png':
            img_converted.save(output_file, 'PNG', quality=95)
        else:
            # JPG 格式
            img_converted.save(output_file, 'JPEG', quality=95)
        
        return 0
        
    except Exception as e:
        print(f"错误: 转换过程失败", file=sys.stderr)
        print(f"详细信息: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc(file=sys.stderr)
        return 1


def main():
    """主入口函数"""
    if len(sys.argv) != 3:
        print("用法: heif_converter.py <input_heif> <output_image>", file=sys.stderr)
        print("支持的输入格式: .heif, .heic", file=sys.stderr)
        print("支持的输出格式: .png, .jpg, .jpeg", file=sys.stderr)
        print("示例: heif_converter.py photo.heic photo.png", file=sys.stderr)
        return 1
    
    input_path = sys.argv[1]
    output_path = sys.argv[2]
    
    return convert_heif_to_image(input_path, output_path)


if __name__ == '__main__':
    sys.exit(main())
