export interface ImageItem {
  id: number;
  title: string;
  tags: string[];
  image_url: string;
  thumbnail_url: string;
  created_at: string;
  score?: number;
}
